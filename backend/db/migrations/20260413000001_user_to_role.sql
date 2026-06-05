-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS app.user_to_role (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES app."user"(id) ON DELETE CASCADE,
    role_id      UUID NOT NULL REFERENCES app.role(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES app.workspace(id) ON DELETE CASCADE,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_user_to_role_updated_at ON app.user_to_role(updated_at);

-- used to load all roles for a user in a workspace on every permission check
CREATE INDEX IF NOT EXISTS idx_user_to_role_user_workspace ON app.user_to_role(user_id, workspace_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS app.idx_user_to_role_user_workspace;
DROP TABLE IF EXISTS app.user_to_role;
-- +goose StatementEnd
