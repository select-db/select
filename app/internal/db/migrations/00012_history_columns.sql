-- +goose Up
-- SQLite forbids a non-constant default (CURRENT_TIMESTAMP) on ADD COLUMN, so the
-- column is added nullable and backfilled; new rows always set created_at explicitly.
ALTER TABLE history ADD COLUMN created_at DATETIME;
UPDATE history SET created_at = CURRENT_TIMESTAMP WHERE created_at IS NULL;
ALTER TABLE history ADD COLUMN workspace_id TEXT NOT NULL DEFAULT '';
ALTER TABLE history ADD COLUMN db_instance_id TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_history_workspace_created_at ON history(workspace_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_history_workspace_created_at;
ALTER TABLE history DROP COLUMN db_instance_id;
ALTER TABLE history DROP COLUMN workspace_id;
ALTER TABLE history DROP COLUMN created_at;
