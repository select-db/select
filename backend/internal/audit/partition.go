package audit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Partition lifecycle for audit.event. The table is LIST-partitioned by domain,
// and each domain is RANGE-partitioned by month. The migration only creates the
// per-domain parents + a DEFAULT catch-all per domain; this manager creates the
// dated monthly partitions ahead of time (so rows never actually land in
// DEFAULT) and drops partitions past their retention window.
//
// Indexes and the principal_hash FK are defined on the top-level partitioned
// table, so Postgres applies them automatically to every monthly partition
// created here — nothing extra to do per partition.
//
// Multi-node: every backend runs maintenance, so the DDL is serialized behind a
// cluster-wide Postgres advisory lock — exactly one node mutates partitions at a
// time (concurrent CREATE/DROP would otherwise race even with IF NOT EXISTS).

const (
	// partitionsAhead is how many future months to pre-create (plus the current
	// month). A generous buffer means a stalled manager still won't push rows
	// into DEFAULT for a while.
	partitionsAhead = 3

	// partitionInterval is how often ongoing maintenance runs.
	partitionInterval = 24 * time.Hour

	// defaultRetention is how long a domain's partitions are kept before being
	// dropped. Override per domain via retentionByDomain.
	defaultRetention = 365 * 24 * time.Hour

	// partitionAdvisoryLockKey scopes the cluster-wide maintenance lock. Value
	// is arbitrary but must be stable and unique to this job ("audit_pa").
	partitionAdvisoryLockKey int64 = 0x61756469745f7061
)

// allDomains is the fixed set of LIST partitions (matches the migration).
var allDomains = []string{DomainQuery, DomainAuth, DomainIAM, DomainDatasource}

// retentionByDomain overrides defaultRetention for specific domains, e.g. keep
// security-relevant streams longer:
//
//	DomainIAM:  3 * 365 * 24 * time.Hour,
//	DomainAuth: 2 * 365 * 24 * time.Hour,
var retentionByDomain = map[string]time.Duration{}

func retentionFor(domain string) time.Duration {
	if r, ok := retentionByDomain[domain]; ok {
		return r
	}
	return defaultRetention
}

// sqlExecQuerier is satisfied by *sql.DB, *sql.Conn, and *sql.Tx, letting the
// helpers run on the lock-pinned connection.
type sqlExecQuerier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// EnsurePartitions runs partition maintenance, waiting for the cluster-wide
// maintenance lock so that on return the current + upcoming partitions are
// guaranteed to exist. Call synchronously at startup, before the first event is
// written, so rows don't fall into DEFAULT.
func EnsurePartitions(ctx context.Context, db *sql.DB) error {
	return withMaintenanceLock(ctx, db, true)
}

// maintainPartitions runs the same maintenance but skips if another node holds
// the lock (best-effort). Use for periodic background runs so just one node does
// the DDL per cycle.
func maintainPartitions(ctx context.Context, db *sql.DB) error {
	return withMaintenanceLock(ctx, db, false)
}

func withMaintenanceLock(ctx context.Context, db *sql.DB, wait bool) error {
	// Advisory locks are session-scoped, so pin one connection for the whole
	// critical section and release explicitly before returning it to the pool
	// (Close alone would not release a session-level lock).
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	if wait {
		if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, partitionAdvisoryLockKey); err != nil {
			return err
		}
	} else {
		var locked bool
		if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, partitionAdvisoryLockKey).Scan(&locked); err != nil {
			return err
		}
		if !locked {
			return nil // another node is handling maintenance this cycle
		}
	}
	defer func() {
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, partitionAdvisoryLockKey)
	}()

	return runMaintenance(ctx, conn)
}

// runMaintenance creates the current + upcoming monthly partitions for every
// domain and drops any past their retention window. Best-effort: per-domain
// failures are collected and returned but don't abort the rest.
func runMaintenance(ctx context.Context, db sqlExecQuerier) error {
	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	var errs []error
	for _, domain := range allDomains {
		for i := 0; i <= partitionsAhead; i++ {
			m := monthStart.AddDate(0, i, 0)
			if err := createMonthlyPartition(ctx, db, domain, m); err != nil {
				errs = append(errs, fmt.Errorf("create %s %s: %w", domain, m.Format("2006-01"), err))
			}
		}
		if err := dropExpiredPartitions(ctx, db, domain, retentionFor(domain), now); err != nil {
			errs = append(errs, fmt.Errorf("drop expired %s: %w", domain, err))
		}
	}
	return errors.Join(errs...)
}

// monthlyPartitionName is the deterministic child name, e.g. event_query_2026_06.
func monthlyPartitionName(domain string, monthStart time.Time) string {
	return fmt.Sprintf("event_%s_%04d_%02d", domain, monthStart.Year(), int(monthStart.Month()))
}

func parentName(domain string) string { return "event_" + domain }

func createMonthlyPartition(ctx context.Context, db sqlExecQuerier, domain string, monthStart time.Time) error {
	monthEnd := monthStart.AddDate(0, 1, 0)
	// Identifiers are built from a fixed domain set + formatted dates, so there
	// is no untrusted input in this statement.
	stmt := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS audit.%s PARTITION OF audit.%s FOR VALUES FROM ('%s') TO ('%s')`,
		monthlyPartitionName(domain, monthStart), parentName(domain),
		monthStart.Format("2006-01-02"), monthEnd.Format("2006-01-02"),
	)
	_, err := db.ExecContext(ctx, stmt)
	return err
}

func dropExpiredPartitions(ctx context.Context, db sqlExecQuerier, domain string, retention time.Duration, now time.Time) error {
	cutoff := now.Add(-retention)
	names, err := listMonthlyPartitions(ctx, db, parentName(domain))
	if err != nil {
		return err
	}

	var errs []error
	for _, name := range names {
		end, ok := partitionMonthEnd(parentName(domain), name)
		if !ok {
			continue // not a dated monthly partition (e.g. the DEFAULT) — never drop
		}
		// Drop only when the partition's whole range is older than the cutoff,
		// i.e. its newest possible row has aged out.
		if !end.After(cutoff) {
			if _, err := db.ExecContext(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS audit.%s`, name)); err != nil {
				errs = append(errs, fmt.Errorf("drop %s: %w", name, err))
			}
		}
	}
	return errors.Join(errs...)
}

// listMonthlyPartitions returns the child partition relnames of a domain parent.
func listMonthlyPartitions(ctx context.Context, db sqlExecQuerier, parent string) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT c.relname
		FROM pg_inherits i
		JOIN pg_class c ON c.oid = i.inhrelid
		JOIN pg_class p ON p.oid = i.inhparent
		JOIN pg_namespace n ON n.oid = p.relnamespace
		WHERE n.nspname = 'audit' AND p.relname = $1`, parent)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// partitionMonthEnd parses a child name like event_query_2026_06 into the
// exclusive upper bound of its range (2026-07-01). Returns ok=false for the
// DEFAULT partition or any name that isn't a dated monthly partition.
func partitionMonthEnd(parent, name string) (time.Time, bool) {
	suffix := strings.TrimPrefix(name, parent+"_")
	if suffix == name || suffix == "default" {
		return time.Time{}, false
	}
	parts := strings.Split(suffix, "_")
	if len(parts) != 2 {
		return time.Time{}, false
	}
	year, err1 := strconv.Atoi(parts[0])
	month, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || month < 1 || month > 12 {
		return time.Time{}, false
	}
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	return start.AddDate(0, 1, 0), true
}
