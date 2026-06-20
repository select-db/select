-- +goose Up
-- +goose StatementBegin
-- Content-addressed snapshot of a principal's identity + authz state at event
-- time. Deduped by hash so a principal that performs thousands of actions with
-- an unchanged permission set is stored once. Keeps the audit trail truthful
-- even after roles/permissions/users later change.
CREATE TABLE IF NOT EXISTS app.audit_principal_snapshot (
    snapshot_hash BYTEA       PRIMARY KEY,        -- sha256 of canonical snapshot JSON
    workspace_id  UUID        NOT NULL REFERENCES app.workspace(id) ON DELETE CASCADE,
    snapshot      JSONB       NOT NULL,           -- principal kind/id, role_ids, permission set
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_audit_principal_snapshot_ws ON app.audit_principal_snapshot(workspace_id);
-- +goose StatementEnd

-- +goose StatementBegin
-- Unified, append-only activity log. category is the first partition key so a
-- high-volume query stream never shares indexes or retention with the
-- security-critical iam/auth streams. Each category is sub-partitioned by month
-- (a DEFAULT range partition catches rows until a partition-management job
-- pre-creates monthly partitions).
CREATE TABLE IF NOT EXISTS app.audit_event (
    id              UUID        NOT NULL DEFAULT gen_random_uuid(),
    workspace_id    UUID        NOT NULL,         -- tenancy boundary; leads every index
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    category        TEXT        NOT NULL,         -- 'query' | 'auth' | 'iam' | 'datasource'
    event_type      TEXT        NOT NULL,         -- 'query.executed', 'iam.permission.upserted', ...
    actor_hash      BYTEA       NOT NULL REFERENCES app.audit_principal_snapshot(snapshot_hash),
    target_type     TEXT,                         -- 'permission' | 'role' | 'user' | 'datasource'
    target_id       UUID,
    target_label    TEXT,                         -- denormalized name at event time
    status          TEXT        NOT NULL,         -- 'ok' | 'error' | 'denied'
    payload         JSONB       NOT NULL DEFAULT '{}'::jsonb,  -- category-specific (plaintext, Tier 0)
    sql_fingerprint BYTEA,                        -- query only; sha256 of normalized SQL (grouping)
    duration_ms     BIGINT,                       -- query only
    row_count       BIGINT,                       -- query only
    client_ip       INET,
    PRIMARY KEY (id, category, occurred_at)       -- partition key must be in the PK
) PARTITION BY LIST (category);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS app.audit_event_query
    PARTITION OF app.audit_event FOR VALUES IN ('query') PARTITION BY RANGE (occurred_at);
CREATE TABLE IF NOT EXISTS app.audit_event_auth
    PARTITION OF app.audit_event FOR VALUES IN ('auth') PARTITION BY RANGE (occurred_at);
CREATE TABLE IF NOT EXISTS app.audit_event_iam
    PARTITION OF app.audit_event FOR VALUES IN ('iam') PARTITION BY RANGE (occurred_at);
CREATE TABLE IF NOT EXISTS app.audit_event_datasource
    PARTITION OF app.audit_event FOR VALUES IN ('datasource') PARTITION BY RANGE (occurred_at);

-- DEFAULT range partitions keep inserts working before a monthly-partition job
-- exists. The job (follow-up) creates dated partitions and drops old ones.
CREATE TABLE IF NOT EXISTS app.audit_event_query_default      PARTITION OF app.audit_event_query      DEFAULT;
CREATE TABLE IF NOT EXISTS app.audit_event_auth_default       PARTITION OF app.audit_event_auth       DEFAULT;
CREATE TABLE IF NOT EXISTS app.audit_event_iam_default        PARTITION OF app.audit_event_iam        DEFAULT;
CREATE TABLE IF NOT EXISTS app.audit_event_datasource_default PARTITION OF app.audit_event_datasource DEFAULT;
-- +goose StatementEnd

-- +goose StatementBegin
-- Every read is workspace-scoped, so every index leads with workspace_id.
CREATE INDEX IF NOT EXISTS idx_audit_event_ws_time
    ON app.audit_event (workspace_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_event_actor
    ON app.audit_event (workspace_id, actor_hash, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_event_target
    ON app.audit_event (workspace_id, target_type, target_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_event_errors
    ON app.audit_event (workspace_id, occurred_at DESC) WHERE status IN ('error', 'denied');
CREATE INDEX IF NOT EXISTS idx_audit_event_fingerprint
    ON app.audit_event (workspace_id, sql_fingerprint);
-- +goose StatementEnd

-- +goose StatementBegin
-- Transactional outbox for security-critical (iam/datasource) events: the event
-- is enqueued durably and a background worker moves it into audit_event. Lets
-- the write stay off the request hot path while surviving a crash.
CREATE TABLE IF NOT EXISTS app.audit_outbox (
    id          BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_json  JSONB       NOT NULL,
    enqueued_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.audit_outbox;
DROP TABLE IF EXISTS app.audit_event CASCADE;
DROP TABLE IF EXISTS app.audit_principal_snapshot CASCADE;
-- +goose StatementEnd
