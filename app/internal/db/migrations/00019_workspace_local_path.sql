-- +goose Up
-- Where this workspace's folder is on this machine. Never synced: it is true of
-- one computer, not of the workspace. A hint for reopening the last folder; the
-- folder's own select.config.json is the authority.
-- +goose StatementBegin
ALTER TABLE workspace ADD COLUMN local_path TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE workspace DROP COLUMN local_path;
-- +goose StatementEnd
