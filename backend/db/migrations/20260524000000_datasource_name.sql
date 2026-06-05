-- +goose Up
-- +goose StatementBegin
ALTER TABLE app.datasource ADD COLUMN name TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.datasource DROP COLUMN name;
-- +goose StatementEnd
