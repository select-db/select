-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS audit;
-- +goose StatementEnd

-- +goose StatementBegin
-- pg_partman manages the monthly partitions and retention;
-- Maintenance runs via pg_cron in the cluster's default
-- Gated on availability so the schema still applies on a 
-- dev/test Postgres that lacks the extension (it
-- falls back to default partitions below).
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_available_extensions WHERE name = 'pg_partman') THEN
        CREATE SCHEMA IF NOT EXISTS partman;
        CREATE EXTENSION IF NOT EXISTS pg_partman SCHEMA partman;
    END IF;
END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
-- Snapshot of a principal's identity + authz state at event time, content-
-- addressed by hash so an unchanged permission set or user is stored once.
CREATE TABLE IF NOT EXISTS audit.principal_snapshot (
    snapshot_hash BYTEA       PRIMARY KEY,        -- sha256 of canonical snapshot JSON
    workspace_id  UUID        NOT NULL REFERENCES app.workspace(id) ON DELETE CASCADE,
    snapshot      JSONB       NOT NULL,           -- principal kind/id, role_ids, permission set
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_principal_snapshot_ws ON audit.principal_snapshot(workspace_id);
-- +goose StatementEnd

-- +goose StatementBegin
-- Unified, append-only activity log. domain is the first partition key so the
-- high-volume query stream never shares indexes or retention with the
-- security-critical iam/auth streams (full event identity = domain.action).
CREATE TABLE IF NOT EXISTS audit.event (
    id                 UUID        NOT NULL DEFAULT gen_random_uuid(),
    workspace_id       UUID        NOT NULL,      -- tenancy boundary; leads every index
    occurred_at        TIMESTAMPTZ NOT NULL DEFAULT now(),  -- when the event happened (set by the app)
    recorded_at        TIMESTAMPTZ NOT NULL DEFAULT now(),  -- when the row was persisted (set by the DB); lags occurred_at via the async/outbox lanes
    domain             TEXT        NOT NULL,      -- 'query' | 'auth' | 'iam' | 'datasource'
    action             TEXT        NOT NULL,      -- 'executed', 'permission.upserted', 'login', ...
    
    principal_hash     BYTEA       NOT NULL REFERENCES audit.principal_snapshot(snapshot_hash),
    principal_id       UUID,                      -- denormalized actor id, for filtering events "by user"
    principal_type     TEXT,                      -- 'user' | 'api_key'
    
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

-- Per-domain LIST partitions, each RANGE-partitioned by month (pg_partman fills
-- the monthly children below).
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
-- Indexes on the parent propagate to every monthly partition pg_partman creates.
-- Every read is workspace-scoped, so every index leads with workspace_id.
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
-- With pg_partman: register each domain's monthly RANGE level (premakes upcoming
-- partitions + a default, drops expired ones per part_config) and set retention.
-- Without it (dev/test): attach one DEFAULT partition per domain so inserts route
-- somewhere, production gets monthly partitions managed by partman instead.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_partman') THEN
        PERFORM partman.create_parent(p_parent_table := 'audit.event_query',      p_control := 'occurred_at', p_interval := '1 month', p_type := 'range');
        PERFORM partman.create_parent(p_parent_table := 'audit.event_auth',       p_control := 'occurred_at', p_interval := '1 month', p_type := 'range');
        PERFORM partman.create_parent(p_parent_table := 'audit.event_iam',        p_control := 'occurred_at', p_interval := '1 month', p_type := 'range');
        PERFORM partman.create_parent(p_parent_table := 'audit.event_datasource', p_control := 'occurred_at', p_interval := '1 month', p_type := 'range');

        -- Retention: drop partitions (don't just detach) older than the window.
        -- One year for all; override per domain to keep security streams longer:
        --   UPDATE partman.part_config SET retention = '3 years'
        --    WHERE parent_table IN ('audit.event_iam', 'audit.event_auth');
        UPDATE partman.part_config
           SET retention = '1 year', retention_keep_table = false
         WHERE parent_table IN ('audit.event_query', 'audit.event_auth', 'audit.event_iam', 'audit.event_datasource');
    ELSE
        CREATE TABLE IF NOT EXISTS audit.event_query_default      PARTITION OF audit.event_query      DEFAULT;
        CREATE TABLE IF NOT EXISTS audit.event_auth_default       PARTITION OF audit.event_auth       DEFAULT;
        CREATE TABLE IF NOT EXISTS audit.event_iam_default        PARTITION OF audit.event_iam        DEFAULT;
        CREATE TABLE IF NOT EXISTS audit.event_datasource_default PARTITION OF audit.event_datasource DEFAULT;
    END IF;
END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
-- Durable outbox for security-critical (iam/datasource) events: enqueued in the
-- request tx, moved into audit.event by a background worker, off the hot path
-- yet crash-safe.
CREATE TABLE IF NOT EXISTS audit.outbox (
    id          BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_json  JSONB       NOT NULL,
    enqueued_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_partman') THEN
        DELETE FROM partman.part_config WHERE parent_table LIKE 'audit.%';
    END IF;
END
$$;
DROP SCHEMA IF EXISTS audit CASCADE;
-- +goose StatementEnd
