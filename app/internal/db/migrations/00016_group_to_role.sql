-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS group_to_role (
    id           TEXT PRIMARY KEY,
    group_id     TEXT NOT NULL,
    role_id      TEXT NOT NULL,
    workspace_id TEXT NOT NULL,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at   DATETIME,
    UNIQUE (group_id, role_id),
    FOREIGN KEY (group_id) REFERENCES "group"(id) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES role(id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id) REFERENCES workspace(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_group_to_role_updated_at ON group_to_role(updated_at);
CREATE INDEX IF NOT EXISTS idx_group_to_role_group_id ON group_to_role(group_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS group_to_role;
-- +goose StatementEnd
