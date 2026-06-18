-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS app.datasource (
    id                 UUID        PRIMARY KEY,  -- matches the local DB instance UUID from the app
    workspace_id       UUID        NOT NULL REFERENCES app.workspace(id) ON DELETE CASCADE,
    db_type            TEXT        NOT NULL,
    encrypted_dsn      BYTEA,       -- kms envelope blob of the DSN
    encrypted_ssh      BYTEA,       -- kms envelope blob of JSON-encoded SSH config
    max_open_conns     INTEGER     NOT NULL DEFAULT 25,
    max_idle_conns     INTEGER     NOT NULL DEFAULT 5,
    conn_max_lifetime  INTEGER     NOT NULL DEFAULT 0,  -- seconds; 0 = no limit
    conn_max_idle_time INTEGER     NOT NULL DEFAULT 0,  -- seconds; 0 = no limit
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (id, workspace_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.datasource;
-- +goose StatementEnd
