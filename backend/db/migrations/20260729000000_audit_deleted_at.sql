-- +goose Up
-- +goose StatementBegin
-- audit.event is append-only, so it lacked the soft-delete column the generated
-- read API filters on. Add it for symmetry with the synced entities (it stays
-- NULL in practice); the REST list/get then apply `deleted_at IS NULL` uniformly.
-- Adding a nullable column to the partitioned parent propagates to every partition.
-- (The default-list sort tags this migration used to carry now live in the single
-- apigen_tags migration, which runs last.)
ALTER TABLE audit.event ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE audit.event DROP COLUMN IF EXISTS deleted_at;
-- +goose StatementEnd
