-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS auth.api_key (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  UUID NOT NULL REFERENCES app.workspace(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    prefix        TEXT NOT NULL UNIQUE,   -- lookup handle
    hashed_key    TEXT NOT NULL UNIQUE,   -- HMAC of the secret, never plaintext
    created_by    UUID REFERENCES app."user"(id) ON DELETE SET NULL,
    expires_at    TIMESTAMPTZ,            -- NULL = no expiry
    last_used_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ
);

-- lookup by prefix on every API-key request
CREATE INDEX IF NOT EXISTS idx_api_key_prefix ON auth.api_key(prefix) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_api_key_workspace_id ON auth.api_key(workspace_id);

-- a key's grant is its bound roles, nothing more
CREATE TABLE IF NOT EXISTS auth.api_key_to_role (
    api_key_id UUID NOT NULL REFERENCES auth.api_key(id) ON DELETE CASCADE,
    role_id    UUID NOT NULL REFERENCES app.role(id) ON DELETE CASCADE,
    PRIMARY KEY (api_key_id, role_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS auth.api_key_to_role;
DROP TABLE IF EXISTS auth.api_key CASCADE;
-- +goose StatementEnd
