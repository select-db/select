# Audit log partitioning & maintenance (operations)

The `audit.event` table is partitioned **LIST by `domain`** → **RANGE by month**.
Monthly partition creation and retention (dropping old months) are handled
**in-database by `pg_partman`**, triggered on a schedule by **`pg_cron`**. The
application contains no partition-management code.

This works the same for the managed deployment and for self-hosted / on-prem
installs, as long as the two extensions are present. Both are available on
self-managed PostgreSQL (OS packages) and on AWS RDS/Aurora, GCP Cloud SQL, and
Azure Flexible Server.

## Prerequisites

- **PostgreSQL 14+**.
- **`pg_partman`** available on the instance. The migration runs
  `CREATE EXTENSION pg_partman` — if the extension files aren't installed, the
  migration fails loudly (a good early signal).
- **`pg_cron`** enabled. It must be loaded at startup, so it needs
  `shared_preload_libraries` **+ a restart**, and it schedules jobs from the
  cluster's *cron database* (not the app database).

Enablement differs per platform:

| Platform | How |
| --- | --- |
| Self-managed | `apt install postgresql-16-partman postgresql-16-cron`; add `shared_preload_libraries='pg_cron'` (and optionally `cron.database_name`) to `postgresql.conf`; restart. |
| AWS RDS / Aurora | Add `pg_cron` to `shared_preload_libraries` in the parameter group; reboot. `pg_partman` and `pg_cron` are supported (PG 12.5+). |
| GCP Cloud SQL | Flags `cloudsql.enable_pg_cron=on`, `cron.database_name=<db>`. |
| Azure Flexible Server | Add `pg_partman` and `pg_cron` to `azure.extensions`; add `pg_cron` to `shared_preload_libraries`; restart. |

## Setup steps

1. **Run migrations** against the app database: `./server migrate:up`.
   This creates the `audit` schema, the partitioned tables, enables
   `pg_partman`, registers the four domain parents via `create_parent`, and sets
   retention (default 1 year; override per domain in `partman.part_config`).

2. **Schedule maintenance.** `pg_partman`'s `run_maintenance_proc()` must run
   periodically (it creates upcoming partitions and drops expired ones). Two ways:

   **a) Automatic (recommended).** Set `AUDIT_CRON_DSN` to a connection string
   for the cluster's **cron database** (the one named by `cron.database_name`;
   often `postgres`, OVH default `defaultdb`). On boot the backend idempotently
   registers the job via `cron.schedule_in_database`, targeting the app database.
   Requires that DSN's role to have `pg_cron` privileges.

   - `AUDIT_CRON_DSN` — DSN to the cron database. Unset ⇒ feature off.
   - `AUDIT_CRON_SCHEDULE` — cron expression (default `17 3 * * *`, daily 03:17).

   **b) Manual.** If the app role shouldn't have cron-database access, run once in
   the cron database:

   ```sql
   SELECT cron.schedule_in_database(
     'audit-partman-maintenance', '17 3 * * *',
     $$CALL partman.run_maintenance_proc()$$,
     '<app_database_name>');
   ```

## Preflight / health

On boot the backend runs a **preflight check** against the app database and logs
a loud `WARNING` if `pg_partman` is missing or the four parents aren't registered
in `partman.part_config`. This turns a silent "maintenance never runs" failure
into an immediate, actionable signal — important for self-hosted databases the
app doesn't control.

The app does **not** fail to start on a failed preflight: with `premake` (default
4 months of partitions created ahead) and the per-domain default partition,
inserts keep working for months even if maintenance is misconfigured. Fix the
prerequisites and the next maintenance run heals it.

## Retention

Retention is per parent in `partman.part_config` (`retention`,
`retention_keep_table = false` to drop). Default is 1 year for all domains.
To keep security streams longer:

```sql
UPDATE partman.part_config SET retention = '3 years'
 WHERE parent_table IN ('audit.event_iam', 'audit.event_auth');
```
