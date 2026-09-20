package audit

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/lib/pq"
)

// Partitions are managed in-DB by pg_partman; the Logger runs its maintenance
// daily (see Start). Raw SQL throughout: these hit catalog/extension objects
// (pg_extension, partman.part_config) the migrations do not describe, so sqlc
// can't type them.

const (
	// maintenanceLockKey serializes maintenance across backend instances.
	maintenanceLockKey = 0x61756469745f706d // "audit_pm"
	maintenanceTimeout = 5 * time.Minute
)

// auditParents are the partitioned parents pg_partman must be managing.
var auditParents = []string{
	"audit.event_query",
	"audit.event_auth",
	"audit.event_iam",
	"audit.event_datasource",
}

// Preflight reports on partition maintenance. With pg_partman it verifies every
// audit parent is registered, surfacing a misconfigured DB loudly. Without
// pg_partman it's a supported mode (the Logger's in-app sweeper handles
// retention), so it logs an info line and returns nil.
func Preflight(ctx context.Context, db *sql.DB) error {
	var hasPartman bool
	if err := db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_partman')`).Scan(&hasPartman); err != nil {
		return fmt.Errorf("audit preflight: checking pg_partman: %w", err)
	}
	if !hasPartman {
		log.Printf("audit: pg_partman not installed, using in-app retention (AUDIT_RETENTION_DAYS). Install pg_partman for partition-based retention at scale")
		return nil
	}

	var managed int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM partman.part_config WHERE parent_table = ANY($1)`, pq.Array(auditParents)).Scan(&managed); err != nil {
		return fmt.Errorf("audit preflight: reading partman.part_config: %w", err)
	}
	if managed < len(auditParents) {
		return fmt.Errorf("audit preflight: only %d/%d audit partition parents are registered in pg_partman. Partitions may stop being created or retained; re-run migrations", managed, len(auditParents))
	}
	return nil
}

func (l *Logger) partitionMaintenance() {
	ctx, cancel := context.WithTimeout(context.Background(), maintenanceTimeout)
	defer cancel()
	if err := runPartitionMaintenance(ctx, l.db); err != nil {
		log.Printf("WARNING: audit: partition maintenance: %v", err)
	}
}

// runPartitionMaintenance premakes upcoming partitions and drops expired ones.
// The advisory lock is session-scoped, so lock, CALL and unlock share one
// connection; an instance that loses the race skips this round.
func runPartitionMaintenance(ctx context.Context, db *sql.DB) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	var locked bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, maintenanceLockKey).Scan(&locked); err != nil {
		return fmt.Errorf("taking lock: %w", err)
	}
	if !locked {
		return nil
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, maintenanceLockKey)
	}()

	// A procedure that commits per partition set, so it runs outside any tx.
	if _, err := conn.ExecContext(ctx, `CALL partman.run_maintenance_proc()`); err != nil {
		return fmt.Errorf("run_maintenance_proc: %w", err)
	}
	return nil
}
