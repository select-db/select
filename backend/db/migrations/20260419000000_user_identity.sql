-- +goose Up
-- +goose StatementBegin
CREATE TABLE app.user_identity (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID NOT NULL REFERENCES app."user"(id) ON DELETE CASCADE,
    provider         TEXT NOT NULL,
    provider_user_id TEXT NOT NULL,
    email            TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, provider_user_id)
);

CREATE INDEX IF NOT EXISTS idx_user_identity_user_id ON app.user_identity(user_id);

-- Backfill: users with a github_id
INSERT INTO app.user_identity (user_id, provider, provider_user_id, email)
SELECT id, 'github', github_id::TEXT, email
FROM app."user"
WHERE github_id IS NOT NULL;


-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.user_identity;
-- +goose StatementEnd
