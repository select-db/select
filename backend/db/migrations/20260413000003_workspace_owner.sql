-- +goose Up
-- +goose StatementBegin
ALTER TABLE app.workspace ADD COLUMN owner_id UUID REFERENCES app."user"(id) ON DELETE SET NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.workspace DROP COLUMN owner_id;
-- +goose StatementEnd
