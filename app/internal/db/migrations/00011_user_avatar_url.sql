-- +goose Up
-- +goose StatementBegin
ALTER TABLE "user" ADD COLUMN avatar_url TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE "user" DROP COLUMN avatar_url;
-- +goose StatementEnd
