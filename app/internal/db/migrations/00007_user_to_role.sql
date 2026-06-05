-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS user_to_role (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL,
    role_id      TEXT NOT NULL,
    workspace_id TEXT NOT NULL,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at   DATETIME,
    UNIQUE (user_id, role_id),
    FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES role(id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id) REFERENCES workspace(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_user_to_role_updated_at ON user_to_role(updated_at);

-- used to load all roles for a user in a workspace on every permission check
CREATE INDEX IF NOT EXISTS idx_user_to_role_user_workspace ON user_to_role(user_id, workspace_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_user_to_role_user_workspace;
DROP TABLE IF EXISTS user_to_role;
-- +goose StatementEnd
