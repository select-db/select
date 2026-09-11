-- +goose Up
-- Who is signed in. Never synced: the other rows in this table are teammates,
-- and which of them is at this keyboard is true of one computer.
--
-- It used to be read off workspace_to_user.current, which made the signed-in
-- user a property of having a workspace open: deleting the last workspace, or
-- signing in before opening a folder, left the app with nobody signed in.
-- +goose StatementBegin
ALTER TABLE user ADD COLUMN current BOOL DEFAULT FALSE;
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE user
SET current = TRUE
WHERE id = (SELECT user_id FROM workspace_to_user WHERE current = TRUE LIMIT 1);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE user DROP COLUMN current;
-- +goose StatementEnd
