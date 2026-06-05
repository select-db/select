-- +goose Up
-- +goose StatementBegin
CREATE TABLE mutation_commit (
    id TEXT PRIMARY KEY,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    operation TEXT NOT NULL,
    table_name TEXT NOT NULL,
    object_id TEXT NOT NULL,
    payload JSON NOT NULL,

    user_id TEXT NOT NULL,
    workspace_id TEXT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id) REFERENCES workspace(id) ON DELETE CASCADE,

    CONSTRAINT unique_sync UNIQUE (operation, table_name, object_id, user_id, workspace_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS mutation_commit;
-- +goose StatementEnd
