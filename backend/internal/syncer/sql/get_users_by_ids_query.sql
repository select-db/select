-- name: GetUsersByIDs :many
SELECT id, name, COALESCE(email, '') AS email, COALESCE(avatar_url, '') AS avatar_url
FROM app."user"
WHERE id::text = ANY($1::text[]);
