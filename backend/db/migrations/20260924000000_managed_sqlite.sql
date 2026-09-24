-- +goose Up
-- Managed SQLite: a datasource with a cellar_id is a SQLite database hosted by
-- that cellar, and the workspace plan sets its quotas.
-- +goose StatementBegin
ALTER TABLE app.workspace
    ADD COLUMN IF NOT EXISTS plan TEXT NOT NULL DEFAULT 'solo'
    CONSTRAINT workspace_plan_check CHECK (plan IN ('solo', 'teams'));
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS app.cellar (
    id         TEXT        PRIMARY KEY CONSTRAINT cellar_id_check CHECK (id ~ '^[a-z0-9][a-z0-9-]*$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- A row is managed exactly when it has a cellar, and then it always has a state.
-- +goose StatementBegin
ALTER TABLE app.datasource
    ADD COLUMN IF NOT EXISTS cellar_id    TEXT REFERENCES app.cellar(id),
    ADD COLUMN IF NOT EXISTS state        TEXT
        CONSTRAINT datasource_state_check CHECK (state IN ('hot', 'cold', 'moving', 'deleting')),
    ADD COLUMN IF NOT EXISTS size_bytes   BIGINT,
    ADD COLUMN IF NOT EXISTS last_used_at TIMESTAMPTZ,
    ADD CONSTRAINT datasource_managed_check CHECK ((cellar_id IS NULL) = (state IS NULL));
-- +goose StatementEnd

-- A retried create must hit a conflict, not make a second db.
-- +goose StatementBegin
CREATE UNIQUE INDEX IF NOT EXISTS datasource_managed_name_key
    ON app.datasource (workspace_id, name) WHERE cellar_id IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS app.datasource_managed_name_key;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE app.datasource
    DROP CONSTRAINT IF EXISTS datasource_managed_check,
    DROP COLUMN IF EXISTS last_used_at,
    DROP COLUMN IF EXISTS size_bytes,
    DROP COLUMN IF EXISTS state,
    DROP COLUMN IF EXISTS cellar_id;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS app.cellar;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE app.workspace DROP COLUMN IF EXISTS plan;
-- +goose StatementEnd
