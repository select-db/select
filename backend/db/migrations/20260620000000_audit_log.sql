-- +goose Up
-- +goose StatementBegin
-- Dedicated schema so the audit tables have their own namespace and can later
-- carry a distinct privilege boundary (e.g. app role INSERT-only on audit.event
-- to enforce an immutable trail; reads restricted to an auditor role). Grants
-- are a follow-up — note audit.outbox needs DELETE for the drainer, so any
-- append-only policy applies to audit.event, not the whole schema.
CREATE SCHEMA IF NOT EXISTS audit;
-- +goose StatementEnd

-- +goose StatementBegin
-- pg_partman manages the monthly RANGE partitions and retention (create_parent
-- calls + part_config below). It must be available on the instance (OVH lists it
-- under supported extensions). The app contains NO partition code; maintenance
-- is run by pg_cron in the cluster's default database — see the provisioning
-- note at the bottom of this file.
CREATE SCHEMA IF NOT EXISTS partman;
CREATE EXTENSION IF NOT EXISTS pg_partman SCHEMA partman;
-- +goose StatementEnd

-- +goose StatementBegin
-- Content-addressed snapshot of a principal's identity + authz state at event
-- time. Deduped by hash so a principal that performs thousands of actions with
-- an unchanged permission set is stored once. Keeps the audit trail truthful
-- even after roles/permissions/users later change.
CREATE TABLE IF NOT EXISTS audit.principal_snapshot (
    snapshot_hash BYTEA       PRIMARY KEY,        -- sha256 of canonical snapshot JSON
    workspace_id  UUID        NOT NULL REFERENCES app.workspace(id) ON DELETE CASCADE,
    snapshot      JSONB       NOT NULL,           -- principal kind/id, role_ids, permission set
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_principal_snapshot_ws ON audit.principal_snapshot(workspace_id);
-- +goose StatementEnd

-- +goose StatementBegin
-- Unified, append-only activity log. domain is the coarse subsystem bucket and
-- the first partition key, so a high-volume query stream never shares indexes
-- or retention with the security-critical iam/auth streams. action is the
-- specific event within a domain (full identity = domain.action). Each domain is
-- sub-partitioned by month, managed by pg_partman.
CREATE TABLE IF NOT EXISTS audit.event (
    id                 UUID        NOT NULL DEFAULT gen_random_uuid(),
    workspace_id       UUID        NOT NULL,      -- tenancy boundary; leads every index
    occurred_at        TIMESTAMPTZ NOT NULL DEFAULT now(),  -- when the event happened (set by the app)
    recorded_at        TIMESTAMPTZ NOT NULL DEFAULT now(),  -- when the row was persisted (set by the DB); lags occurred_at via the async/outbox lanes
    domain             TEXT        NOT NULL,      -- 'query' | 'auth' | 'iam' | 'datasource'
    action             TEXT        NOT NULL,      -- 'executed', 'permission.upserted', 'login', ...
    principal_hash     BYTEA       NOT NULL REFERENCES audit.principal_snapshot(snapshot_hash),
    target_type        TEXT,                      -- 'permission' | 'role' | 'user' | 'datasource'
    target_id          UUID,
    target_label       TEXT,                      -- denormalized name at event time
    status             TEXT        NOT NULL,      -- 'success' | 'error' | 'denied'
    payload            JSONB       NOT NULL DEFAULT '{}'::jsonb,  -- domain-specific (plaintext, Tier 0)
    duration_ms        BIGINT,                    -- query only
    returned_row_count BIGINT,                    -- query only
    client_ip          INET,
    PRIMARY KEY (id, domain, occurred_at)         -- partition key must be in the PK
) PARTITION BY LIST (domain);

-- Fixed per-domain LIST partitions, each RANGE-partitioned by month. pg_partman
-- fills in the monthly child partitions + a default for each (below).
CREATE TABLE IF NOT EXISTS audit.event_query
    PARTITION OF audit.event FOR VALUES IN ('query') PARTITION BY RANGE (occurred_at);
CREATE TABLE IF NOT EXISTS audit.event_auth
    PARTITION OF audit.event FOR VALUES IN ('auth') PARTITION BY RANGE (occurred_at);
CREATE TABLE IF NOT EXISTS audit.event_iam
    PARTITION OF audit.event FOR VALUES IN ('iam') PARTITION BY RANGE (occurred_at);
CREATE TABLE IF NOT EXISTS audit.event_datasource
    PARTITION OF audit.event FOR VALUES IN ('datasource') PARTITION BY RANGE (occurred_at);
-- +goose StatementEnd

-- +goose StatementBegin
-- Indexes on the top-level partitioned table propagate automatically to every
-- monthly partition pg_partman creates. Every read is workspace-scoped, so every
-- index leads with workspace_id.
CREATE INDEX IF NOT EXISTS idx_event_ws_time
    ON audit.event (workspace_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_event_principal
    ON audit.event (workspace_id, principal_hash, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_event_target
    ON audit.event (workspace_id, target_type, target_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_event_errors
    ON audit.event (workspace_id, occurred_at DESC) WHERE status IN ('error', 'denied');
-- +goose StatementEnd

-- +goose StatementBegin
-- Hand each domain's monthly RANGE level to pg_partman: it pre-creates upcoming
-- partitions (premake) + a default, and drops expired ones per part_config.
SELECT partman.create_parent(p_parent_table := 'audit.event_query',      p_control := 'occurred_at', p_interval := '1 month', p_type := 'range');
SELECT partman.create_parent(p_parent_table := 'audit.event_auth',       p_control := 'occurred_at', p_interval := '1 month', p_type := 'range');
SELECT partman.create_parent(p_parent_table := 'audit.event_iam',        p_control := 'occurred_at', p_interval := '1 month', p_type := 'range');
SELECT partman.create_parent(p_parent_table := 'audit.event_datasource', p_control := 'occurred_at', p_interval := '1 month', p_type := 'range');

-- Retention: drop partitions (don't just detach) older than the window. Default
-- one year for all; override per domain to keep security streams longer, e.g.:
--   UPDATE partman.part_config SET retention = '3 years'
--    WHERE parent_table IN ('audit.event_iam', 'audit.event_auth');
UPDATE partman.part_config
   SET retention = '1 year', retention_keep_table = false
 WHERE parent_table IN ('audit.event_query', 'audit.event_auth', 'audit.event_iam', 'audit.event_datasource');
-- +goose StatementEnd

-- +goose StatementBegin
-- Transactional outbox for security-critical (iam/datasource) events: the event
-- is enqueued durably and a background worker moves it into audit.event. Lets
-- the write stay off the request hot path while surviving a crash.
CREATE TABLE IF NOT EXISTS audit.outbox (
    id          BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_json  JSONB       NOT NULL,
    enqueued_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- Provisioning note (one-time, NOT runnable from this app-DB migration):
-- pg_cron's metadata lives in the cluster's default database, so schedule the
-- maintenance job there, targeting this app database:
--
--   SELECT cron.schedule_in_database(
--     'audit-partman-maintenance', '@daily',
--     $$CALL partman.run_maintenance_proc()$$,
--     '<app_database_name>'   -- OVH default cluster DB is 'defaultdb'
--   );
--
-- Until that job exists, premake gives ~4 months of runway before rows would
-- fall into a default partition.

-- +goose Down
-- +goose StatementBegin
DELETE FROM partman.part_config WHERE parent_table LIKE 'audit.%';
DROP SCHEMA IF EXISTS audit CASCADE;
-- +goose StatementEnd
