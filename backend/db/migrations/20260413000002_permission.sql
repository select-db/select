-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS app.permission (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role_id         UUID NOT NULL REFERENCES app.role(id) ON DELETE CASCADE,
    workspace_id    UUID NOT NULL REFERENCES app.workspace(id) ON DELETE CASCADE,
    -- NULL across all four db fields = app-level rule (workspace settings, user management, etc.)
    -- '*' in schema/table/column = wildcard (applies to all at that level)
    db_instance_id  TEXT,
    schema_name     TEXT,
    table_name      TEXT,
    column_name     TEXT,
    action          TEXT NOT NULL,
    effect          TEXT NOT NULL DEFAULT 'deny',
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_permission_role_id ON app.permission(role_id);
CREATE INDEX IF NOT EXISTS idx_permission_workspace_id ON app.permission(workspace_id);
CREATE INDEX IF NOT EXISTS idx_permission_updated_at ON app.permission(updated_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS app.idx_permission_role_id;
DROP TABLE IF EXISTS app.permission;
-- +goose StatementEnd
