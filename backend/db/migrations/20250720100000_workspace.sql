-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS app.workspace (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    git_remote_url TEXT,
    
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

-- app.workspace: GetWorkspacesForUserSince filters by updated_at > $2
CREATE INDEX IF NOT EXISTS idx_workspace_updated_at ON app.workspace(updated_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS app.idx_workspace_updated_at;
DROP TABLE IF EXISTS app.workspace CASCADE;
-- +goose StatementEnd
