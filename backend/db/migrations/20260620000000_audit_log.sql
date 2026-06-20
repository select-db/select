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
-- Unified, append-only activity log. category is the first partition key so a
-- high-volume query stream never shares indexes or retention with the
-- security-critical iam/auth streams. Each category is sub-partitioned by month
-- (a DEFAULT range partition catches rows until a partition-management job
-- pre-creates monthly partitions).
CREATE TABLE IF NOT EXISTS audit.event (
    id              UUID        NOT NULL DEFAULT gen_random_uuid(),
    workspace_id    UUID        NOT NULL,         -- tenancy boundary; leads every index
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    category        TEXT        NOT NULL,         -- 'query' | 'auth' | 'iam' | 'datasource'
    event_type      TEXT        NOT NULL,         -- 'query.executed', 'iam.permission.upserted', ...
    actor_hash      BYTEA       NOT NULL REFERENCES audit.principal_snapshot(snapshot_hash),
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
CREATE TABLE IF NOT EXISTS audit.event_query
    PARTITION OF audit.event FOR VALUES IN ('query') PARTITION BY RANGE (occurred_at);
CREATE TABLE IF NOT EXISTS audit.event_auth
    PARTITION OF audit.event FOR VALUES IN ('auth') PARTITION BY RANGE (occurred_at);
CREATE TABLE IF NOT EXISTS audit.event_iam
    PARTITION OF audit.event FOR VALUES IN ('iam') PARTITION BY RANGE (occurred_at);
CREATE TABLE IF NOT EXISTS audit.event_datasource
    PARTITION OF audit.event FOR VALUES IN ('datasource') PARTITION BY RANGE (occurred_at);

-- DEFAULT range partitions keep inserts working before a monthly-partition job
-- exists. The job (follow-up) creates dated partitions and drops old ones.
CREATE TABLE IF NOT EXISTS audit.event_query_default      PARTITION OF audit.event_query      DEFAULT;
CREATE TABLE IF NOT EXISTS audit.event_auth_default       PARTITION OF audit.event_auth       DEFAULT;
CREATE TABLE IF NOT EXISTS audit.event_iam_default        PARTITION OF audit.event_iam        DEFAULT;
CREATE TABLE IF NOT EXISTS audit.event_datasource_default PARTITION OF audit.event_datasource DEFAULT;
-- +goose StatementEnd

-- +goose StatementBegin
-- Every read is workspace-scoped, so every index leads with workspace_id.
CREATE INDEX IF NOT EXISTS idx_event_ws_time
    ON audit.event (workspace_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_event_actor
    ON audit.event (workspace_id, actor_hash, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_event_target
    ON audit.event (workspace_id, target_type, target_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_event_errors
    ON audit.event (workspace_id, occurred_at DESC) WHERE status IN ('error', 'denied');
CREATE INDEX IF NOT EXISTS idx_event_fingerprint
    ON audit.event (workspace_id, sql_fingerprint);
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

-- +goose Down
-- +goose StatementBegin
DROP SCHEMA IF EXISTS audit CASCADE;
-- +goose StatementEnd
