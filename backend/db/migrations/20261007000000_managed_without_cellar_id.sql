-- +goose Up
-- A managed database belongs to no cellar: any cellar mounts it from the bucket. A row is
-- managed exactly when it has a state.
-- +goose StatementBegin
ALTER TABLE app.datasource DROP CONSTRAINT IF EXISTS datasource_managed_check;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE app.datasource DROP COLUMN IF EXISTS cellar_id;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE app.datasource ADD CONSTRAINT datasource_managed_check CHECK (
    state IS NULL
    OR (
        state IN ('hot', 'cold', 'moving', 'deleting')
        AND db_type = 'sqlite'
        AND encrypted_dsn IS NULL
        AND COALESCE(size_bytes, 0) >= 0
    )
);
-- +goose StatementEnd

-- +goose Down
-- The rollback is lossy: the column comes back with one name for every managed row.
-- +goose StatementBegin
ALTER TABLE app.datasource DROP CONSTRAINT IF EXISTS datasource_managed_check;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE app.datasource ADD COLUMN IF NOT EXISTS cellar_id TEXT;
-- +goose StatementEnd
-- +goose StatementBegin
UPDATE app.datasource SET cellar_id = 'localhost' WHERE state IS NOT NULL;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE app.datasource ADD CONSTRAINT datasource_managed_check CHECK (
    (cellar_id IS NULL AND state IS NULL)
    OR (
        cellar_id ~ '^[a-z0-9][a-z0-9-]*$'
        AND state IS NOT NULL
        AND state IN ('hot', 'cold', 'moving', 'deleting')
        AND db_type = 'sqlite'
        AND encrypted_dsn IS NULL
        AND COALESCE(size_bytes, 0) >= 0
    )
);
-- +goose StatementEnd
