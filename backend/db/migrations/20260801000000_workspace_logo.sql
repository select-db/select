-- +goose Up
-- +goose StatementBegin
ALTER TABLE app.workspace ADD COLUMN IF NOT EXISTS logo TEXT;
-- +goose StatementEnd

-- The logo column holds the base64 of a PNG produced by our own encoder
-- (see backend/internal/workspace/logo.go), never bytes supplied by a client.
-- Because the format is fixed, the database can enforce it: every base64 PNG
-- starts with the encoding of the PNG magic number (89 50 4E 47 0D 0A 1A 0A),
-- which is always "iVBORw0KGgo". The charset class rejects anything that is not
-- base64 — including a "data:image/..." prefix, so a row can never carry its own
-- MIME type into an <img src>.
-- +goose StatementBegin
ALTER TABLE app.workspace DROP CONSTRAINT IF EXISTS workspace_logo_check;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE app.workspace ADD CONSTRAINT workspace_logo_check CHECK (
    logo IS NULL
    OR (
        octet_length(logo) <= 98304
        AND logo ~ '^iVBORw0KGgo[A-Za-z0-9+/]*={0,2}$'
    )
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.workspace DROP CONSTRAINT IF EXISTS workspace_logo_check;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE app.workspace DROP COLUMN IF EXISTS logo;
-- +goose StatementEnd
