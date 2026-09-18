-- +goose Up
-- +goose StatementBegin
-- "group" is a reserved word in SQLite, hence the quoting throughout.
CREATE TABLE IF NOT EXISTS "group" (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    name         TEXT NOT NULL,
    source       TEXT NOT NULL DEFAULT 'local',
    external_id  TEXT,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at   DATETIME,
    FOREIGN KEY (workspace_id) REFERENCES workspace(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_group_updated_at ON "group"(updated_at);
CREATE INDEX IF NOT EXISTS idx_group_workspace_id ON "group"(workspace_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS "group";
-- +goose StatementEnd
