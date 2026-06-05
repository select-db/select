-- +goose Up
-- +goose StatementBegin
ALTER TABLE "user" ADD COLUMN email TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE "user" DROP COLUMN email;
-- +goose StatementEnd
