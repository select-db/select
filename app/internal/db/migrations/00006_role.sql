-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS role (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    name         TEXT NOT NULL,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at   DATETIME,
    FOREIGN KEY (workspace_id) REFERENCES workspace(id) ON DELETE CASCADE,
    UNIQUE (workspace_id, name)
);

CREATE INDEX IF NOT EXISTS idx_role_updated_at ON role(updated_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS role;
-- +goose StatementEnd
