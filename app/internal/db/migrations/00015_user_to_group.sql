-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS user_to_group (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL,
    group_id     TEXT NOT NULL,
    workspace_id TEXT NOT NULL,
    source       TEXT NOT NULL DEFAULT 'local',
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at   DATETIME,
    UNIQUE (user_id, group_id),
    FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE,
    FOREIGN KEY (group_id) REFERENCES "group"(id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id) REFERENCES workspace(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_user_to_group_updated_at ON user_to_group(updated_at);
CREATE INDEX IF NOT EXISTS idx_user_to_group_user_workspace ON user_to_group(user_id, workspace_id);
CREATE INDEX IF NOT EXISTS idx_user_to_group_group_id ON user_to_group(group_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_user_to_group_user_workspace;
DROP TABLE IF EXISTS user_to_group;
-- +goose StatementEnd
