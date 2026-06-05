-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS app."user" (
    id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    github_id BIGINT,
    name TEXT,
    email TEXT NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app."user" CASCADE;
-- +goose StatementEnd
