-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS permission (
    id              TEXT PRIMARY KEY,
    role_id         TEXT NOT NULL,
    workspace_id    TEXT NOT NULL,
    -- NULL across all four db fields = app-level rule (workspace settings, user management, etc.)
    -- '*' in schema/table/column = wildcard (applies to all at that level)
    db_instance_id  TEXT,
    schema_name     TEXT,
    table_name      TEXT,
    column_name     TEXT,
    action          TEXT NOT NULL,
    effect          TEXT NOT NULL DEFAULT 'deny',
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at      DATETIME,
    FOREIGN KEY (role_id) REFERENCES role(id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id) REFERENCES workspace(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_permission_updated_at ON permission(updated_at);

-- SQLite treats NULLs as distinct in UNIQUE constraints, so we use a COALESCE index
-- to make NULLs equal and prevent duplicate rules.
CREATE UNIQUE INDEX IF NOT EXISTS idx_permission_unique
    ON permission(role_id, COALESCE(db_instance_id, ''), COALESCE(schema_name, ''), COALESCE(table_name, ''), COALESCE(column_name, ''), action);

CREATE INDEX IF NOT EXISTS idx_permission_role_id ON permission(role_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_permission_unique;
DROP INDEX IF EXISTS idx_permission_role_id;
DROP TABLE IF EXISTS permission;
-- +goose StatementEnd
