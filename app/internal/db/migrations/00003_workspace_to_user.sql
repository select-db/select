-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS workspace_to_user (
    id TEXT PRIMARY KEY,
    current BOOLEAN DEFAULT FALSE,

    workspace_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    FOREIGN KEY(workspace_id) REFERENCES workspace(id) ON DELETE CASCADE,
    FOREIGN KEY(user_id) REFERENCES user(id) ON DELETE CASCADE,

    UNIQUE(user_id, workspace_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS workspace_to_user;
-- +goose StatementEnd
