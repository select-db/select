-- +goose Up
ALTER TABLE history ADD COLUMN workspace_id TEXT NOT NULL DEFAULT '';
ALTER TABLE history ADD COLUMN db_instance_id TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_history_workspace_created_at ON history(workspace_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_history_workspace_created_at;
ALTER TABLE history DROP COLUMN db_instance_id;
ALTER TABLE history DROP COLUMN workspace_id;
