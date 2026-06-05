-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS app.workspace_to_user (
    id UUID PRIMARY KEY,
    workspace_id UUID NOT NULL REFERENCES app.workspace(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES app."user"(id) ON DELETE CASCADE,
    
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

-- app.workspace_to_user: middleware loads workspace IDs by user_id on every request
CREATE INDEX IF NOT EXISTS idx_workspace_to_user_user_id ON app.workspace_to_user(user_id);

CREATE INDEX IF NOT EXISTS idx_workspace_to_user_workspace_id ON app.workspace_to_user(workspace_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS app.idx_workspace_to_user_workspace_id;
DROP TABLE IF EXISTS app.workspace_to_user;
-- +goose StatementEnd
