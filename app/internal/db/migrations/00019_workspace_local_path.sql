-- +goose Up
-- Where this workspace's folder is on this machine. Local only: it is never
-- pushed and never pulled, because it is true of one computer and not of the
-- workspace. The folder itself is the source of truth -- its select.config.json
-- says which workspace it is -- and this column only remembers where to look so
-- the last folder can be reopened without asking again.
-- +goose StatementBegin
ALTER TABLE workspace ADD COLUMN local_path TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE workspace DROP COLUMN local_path;
-- +goose StatementEnd
