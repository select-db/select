-- +goose Up
-- The app no longer configures a workspace's repository. A workspace is a
-- folder the user opened, and if that folder is a clone then git already knows
-- where it came from: `git remote get-url origin` is the answer, read from the
-- working tree rather than mirrored into a column that could disagree with it.
-- +goose StatementBegin
ALTER TABLE workspace DROP COLUMN git_remote_url;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE workspace ADD COLUMN git_remote_url TEXT;
-- +goose StatementEnd
