-- +goose Up
-- The local mirror of app.workspace.logo on the backend: base64 of a 128x128
-- PNG. The same CHECK is enforced here so a corrupted pull can never land a
-- value the UI would render as anything other than a PNG. SQLite has no regex,
-- so the base64 charset is expressed as a negated GLOB class.
-- +goose StatementBegin
ALTER TABLE workspace ADD COLUMN logo TEXT CHECK (
    logo IS NULL
    OR (
        length(logo) <= 98304
        AND logo GLOB 'iVBORw0KGgo*'
        AND logo NOT GLOB '*[^A-Za-z0-9+/=]*'
    )
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE workspace DROP COLUMN logo;
-- +goose StatementEnd
