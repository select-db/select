-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS app;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP SCHEMA IF EXISTS app CASCADE;
-- +goose StatementEnd
