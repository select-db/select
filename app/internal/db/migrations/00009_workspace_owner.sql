-- +goose Up
-- +goose StatementBegin
ALTER TABLE workspace ADD COLUMN owner_id TEXT REFERENCES user(id) ON DELETE SET NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE workspace DROP COLUMN owner_id;
-- +goose StatementEnd
