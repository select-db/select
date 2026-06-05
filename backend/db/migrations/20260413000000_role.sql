-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS app.role (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES app.workspace(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_role_updated_at ON app.role(updated_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.role CASCADE;
-- +goose StatementEnd
