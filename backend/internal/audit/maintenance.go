package audit

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"
)

// Partition maintenance is handled in-database by pg_partman; pg_cron triggers
// it. This file provides two boot-time helpers:
//
//   - Preflight: verify the machinery is actually in place (pg_partman present,
//     our parents registered), so a misconfigured self-hosted database fails
//     loudly instead of silently never running maintenance.
//   - EnsureMaintenanceSchedule: self-provision the pg_cron job on boot (like
//     running migrations), since cron.schedule_in_database lives only in the
//     cluster's cron database and can't be created from an app-DB migration.

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

// Preflight checks, against the app database, that partition maintenance can
// actually work: pg_partman installed and all audit parents registered in
// part_config. Returns a descriptive error listing what's wrong; the caller
// decides whether to warn or refuse to start.
func Preflight(ctx context.Context, db *sql.DB) error {
	var hasPartman bool
	if err := db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_partman')`).Scan(&hasPartman); err != nil {
		return fmt.Errorf("audit preflight: checking pg_partman: %w", err)
	}
	if !hasPartman {
		return fmt.Errorf("audit preflight: pg_partman is not installed — partition creation/retention will not run; install the extension and run migrations (migrate:up)")
	}

	var managed int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM partman.part_config WHERE parent_table = ANY($1)`, pq.Array(auditParents)).Scan(&managed); err != nil {
		return fmt.Errorf("audit preflight: reading partman.part_config: %w", err)
	}
	if managed < len(auditParents) {
		return fmt.Errorf("audit preflight: only %d/%d audit partition parents are registered in pg_partman — partitions may stop being created or retained; re-run migrations", managed, len(auditParents))
	}
	return nil
}

// EnsureMaintenanceSchedule idempotently registers the pg_cron job that runs
// pg_partman maintenance. It is a no-op (returns nil) when cronDSN is empty.
//
// pg_cron's scheduling functions exist only in the cluster's cron database, so
// this opens a dedicated connection to cronDSN and schedules the job to run in
// the app's own database (resolved via current_database on appDB). Registering
// by a fixed job name makes it an upsert, so it is safe to run on every boot.
//
// Requires the cronDSN role to have pg_cron privileges. If it doesn't (common on
// locked-down managed databases), leave cronDSN empty and provision the job out
// of band — see the on-prem runbook.
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
		return fmt.Errorf("audit cron: pg_cron is not installed in the cron database — enable it (shared_preload_libraries + restart) or schedule maintenance manually")
	}

	// schedule_in_database upserts by job name, so re-running on boot is safe.
	if _, err := cronDB.ExecContext(ctx,
		`SELECT cron.schedule_in_database($1, $2, $3, $4)`,
		maintenanceJobName, schedule, maintenanceCommand, targetDB); err != nil {
		return fmt.Errorf("audit cron: scheduling maintenance job: %w", err)
	}
	return nil
}
