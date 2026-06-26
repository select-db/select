package audit

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"
)

// Partitions are managed in-DB by pg_partman, triggered by pg_cron. These
// boot-time helpers verify that setup (Preflight) and self-provision the cron
// job (EnsureMaintenanceSchedule). Raw SQL throughout: they hit catalog/extension
// objects (pg_extension, partman.part_config, pg_cron) not in schema.sql, so
// sqlc can't type them.

const (
	maintenanceJobName  = "audit-partman-maintenance"
	maintenanceCommand  = "CALL partman.run_maintenance_proc()"
	defaultCronSchedule = "17 3 * * *" // daily 03:17, off-peak
)

// auditParents are the partitioned parents pg_partman must be managing.
var auditParents = []string{
	"audit.event_query",
	"audit.event_auth",
	"audit.event_iam",
	"audit.event_datasource",
}

// Preflight verifies maintenance can run: pg_partman installed and all audit
// parents registered. Surfaces a misconfigured self-hosted DB loudly instead of
// silently never running maintenance.
func Preflight(ctx context.Context, db *sql.DB) error {
	var hasPartman bool
	if err := db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_partman')`).Scan(&hasPartman); err != nil {
		return fmt.Errorf("audit preflight: checking pg_partman: %w", err)
	}
	if !hasPartman {
		return fmt.Errorf("audit preflight: pg_partman is not installed. Partition creation/retention will not run; install the extension and run migrations (migrate:up)")
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

// EnsureMaintenanceSchedule registers the pg_cron maintenance job; no-op when
// cronDSN is empty. pg_cron's functions live only in the cluster's cron DB, so
// it can't be a migration: this connects to cronDSN and schedules the job to run
// in the app DB. Upserts by job name (safe on every boot). Needs the cronDSN role
// to have pg_cron privileges; otherwise leave cronDSN empty and provision by hand.
func EnsureMaintenanceSchedule(ctx context.Context, appDB *sql.DB, cronDSN, schedule string) error {
	if cronDSN == "" {
		return nil
	}
	if schedule == "" {
		schedule = defaultCronSchedule
	}

	var targetDB string
	if err := appDB.QueryRowContext(ctx, `SELECT current_database()`).Scan(&targetDB); err != nil {
		return fmt.Errorf("audit cron: resolving target database: %w", err)
	}

	cronDB, err := sql.Open("postgres", cronDSN)
	if err != nil {
		return fmt.Errorf("audit cron: opening cron database: %w", err)
	}
	defer func() { _ = cronDB.Close() }()
	if err := cronDB.PingContext(ctx); err != nil {
		return fmt.Errorf("audit cron: connecting to cron database: %w", err)
	}

	var hasCron bool
	if err := cronDB.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_cron')`).Scan(&hasCron); err != nil {
		return fmt.Errorf("audit cron: checking pg_cron: %w", err)
	}
	if !hasCron {
		return fmt.Errorf("audit cron: pg_cron is not installed in the cron database. Enable it (shared_preload_libraries + restart) or schedule maintenance manually")
	}

	// schedule_in_database upserts by job name, so re-running on boot is safe.
	if _, err := cronDB.ExecContext(ctx,
		`SELECT cron.schedule_in_database($1, $2, $3, $4)`,
		maintenanceJobName, schedule, maintenanceCommand, targetDB); err != nil {
		return fmt.Errorf("audit cron: scheduling maintenance job: %w", err)
	}
	return nil
}
