-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS auth;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP SCHEMA IF EXISTS auth CASCADE;
-- +goose StatementEnd
