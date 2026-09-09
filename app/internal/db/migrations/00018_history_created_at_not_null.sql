-- +goose Up
-- 00012 could only add created_at nullable: SQLite rejects a non-constant
-- default on ADD COLUMN. It backfilled every row and every insert sets the
-- column, so NOT NULL is what it has always meant. SQLite has no ALTER COLUMN,
-- so saying it means rebuilding the table.

-- +goose StatementBegin
CREATE TABLE history_new (
    id TEXT PRIMARY KEY,

    statement TEXT NOT NULL,
    affected_rows INTEGER,
    row_count INTEGER,
    duration_ms INTEGER,
    errors TEXT NOT NULL DEFAULT '[]',

    uri TEXT NOT NULL DEFAULT '',
    dsn TEXT NOT NULL DEFAULT '',

    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    workspace_id TEXT NOT NULL DEFAULT '',
    db_instance_id TEXT NOT NULL DEFAULT ''
);
-- +goose StatementEnd

-- +goose StatementBegin
INSERT INTO history_new (
    id, statement, affected_rows, row_count, duration_ms, errors,
    uri, dsn, created_at, workspace_id, db_instance_id
)
SELECT
    id, statement, affected_rows, row_count, duration_ms, errors,
    uri, dsn, COALESCE(created_at, CURRENT_TIMESTAMP), workspace_id, db_instance_id
FROM history;
-- +goose StatementEnd

DROP TABLE history;
ALTER TABLE history_new RENAME TO history;
CREATE INDEX idx_history_workspace_created_at ON history(workspace_id, created_at DESC);

-- +goose Down
-- +goose StatementBegin
CREATE TABLE history_old (
    id TEXT PRIMARY KEY,

    statement TEXT NOT NULL,
    affected_rows INTEGER,
    row_count INTEGER,
    duration_ms INTEGER,
    errors TEXT NOT NULL DEFAULT '[]',

    uri TEXT NOT NULL DEFAULT '',
    dsn TEXT NOT NULL DEFAULT '',

    created_at DATETIME,
    workspace_id TEXT NOT NULL DEFAULT '',
    db_instance_id TEXT NOT NULL DEFAULT ''
);
-- +goose StatementEnd

-- +goose StatementBegin
INSERT INTO history_old (
    id, statement, affected_rows, row_count, duration_ms, errors,
    uri, dsn, created_at, workspace_id, db_instance_id
)
SELECT
    id, statement, affected_rows, row_count, duration_ms, errors,
    uri, dsn, created_at, workspace_id, db_instance_id
FROM history;
-- +goose StatementEnd

DROP TABLE history;
ALTER TABLE history_old RENAME TO history;
CREATE INDEX idx_history_workspace_created_at ON history(workspace_id, created_at DESC);
