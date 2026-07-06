-- +goose Up
-- +goose StatementBegin
ALTER TABLE workspace ADD COLUMN statement_timeout_ms INTEGER NOT NULL DEFAULT 30000;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE workspace ADD COLUMN max_result_size_mb INTEGER NOT NULL DEFAULT 100;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE workspace DROP COLUMN statement_timeout_ms;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE workspace DROP COLUMN max_result_size_mb;
-- +goose StatementEnd
