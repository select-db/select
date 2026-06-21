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

// EnsurePartitions creates the current + upcoming monthly partitions for every
// domain and drops any past their retention window. It is idempotent (safe to
// call at startup and on a schedule) and best-effort: per-domain failures are
// collected and returned but don't abort the rest.
//
// Call this synchronously before the first event is written so partitions exist
// up front and rows don't fall into DEFAULT.
func EnsurePartitions(ctx context.Context, db *sql.DB) error {
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

func createMonthlyPartition(ctx context.Context, db *sql.DB, domain string, monthStart time.Time) error {
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

func dropExpiredPartitions(ctx context.Context, db *sql.DB, domain string, retention time.Duration, now time.Time) error {
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
func listMonthlyPartitions(ctx context.Context, db *sql.DB, parent string) ([]string, error) {
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
