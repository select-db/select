-- +goose Up
-- +goose StatementBegin
CREATE TABLE history (
    id TEXT PRIMARY KEY,

    statement TEXT NOT NULL,
    affected_rows INTEGER,
    row_count INTEGER,
    duration_ms INTEGER,
    errors TEXT NOT NULL DEFAULT '[]',

    uri TEXT NOT NULL DEFAULT "",
    dsn TEXT NOT NULL DEFAULT ""
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS history;
-- +goose StatementEnd
