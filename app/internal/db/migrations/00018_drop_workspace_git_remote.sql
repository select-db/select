-- +goose Up
-- The folder's own `git remote get-url origin` is the remote. A mirror here
-- could disagree with it.
-- +goose StatementBegin
ALTER TABLE workspace DROP COLUMN git_remote_url;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE workspace ADD COLUMN git_remote_url TEXT;
-- +goose StatementEnd
