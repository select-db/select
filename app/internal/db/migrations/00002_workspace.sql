-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS workspace (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    git_remote_url TEXT,

    last_pulled_at DATETIME
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS workspace;
-- +goose StatementEnd
