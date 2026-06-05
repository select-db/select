-- +goose Up
-- +goose StatementBegin
CREATE UNIQUE INDEX IF NOT EXISTS idx_workspace_to_user_unique
    ON app.workspace_to_user(user_id, workspace_id)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS app.idx_workspace_to_user_unique;
-- +goose StatementEnd
