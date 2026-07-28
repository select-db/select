-- +goose Up
-- +goose StatementBegin
-- audit.event is append-only, so it lacked the soft-delete column the generated
-- read API filters on. Add it for symmetry with the synced entities (it stays
-- NULL in practice); the REST list/get then apply `deleted_at IS NULL` uniformly.
-- Adding a nullable column to the partitioned parent propagates to every partition.
ALTER TABLE audit.event ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
-- +goose StatementEnd

-- +goose StatementBegin
-- @app.sort marks a resource's default list sort. The log has no updated_at, so
-- it sorts by event time, newest first; the synced entities default to their
-- name, a friendlier listing than updated_at. Appended to occurred_at's existing
-- description; role.name / group.name had no column comment before.
COMMENT ON COLUMN audit.event.occurred_at IS 'When the event occurred (event time). @app.sort desc';
COMMENT ON COLUMN app.role.name           IS 'Name of the role. @app.sort';
COMMENT ON COLUMN app."group".name        IS 'Name of the group. @app.sort';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
COMMENT ON COLUMN audit.event.occurred_at IS 'When the event occurred (event time).';
COMMENT ON COLUMN app.role.name           IS NULL;
COMMENT ON COLUMN app."group".name        IS NULL;
ALTER TABLE audit.event DROP COLUMN IF EXISTS deleted_at;
-- +goose StatementEnd
