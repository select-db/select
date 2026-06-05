-- +goose Up
-- +goose StatementBegin
CREATE TABLE auth.refresh_token (
    hashed_token TEXT PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES app."user"(id) ON DELETE CASCADE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now(),
    issued_ip INET
);

-- auth.refresh_token: revocation / delete all tokens for user
CREATE INDEX IF NOT EXISTS idx_refresh_token_user_id ON auth.refresh_token(user_id);

-- auth.refresh_token: lookup by (hashed_token, user_id) on every authenticated request
CREATE INDEX IF NOT EXISTS idx_refresh_token_hashed_user ON auth.refresh_token(hashed_token, user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_refresh_token_user_id;
DROP INDEX IF EXISTS idx_refresh_token_hashed_user;
DROP TABLE IF EXISTS auth.refresh_token;
-- +goose StatementEnd
