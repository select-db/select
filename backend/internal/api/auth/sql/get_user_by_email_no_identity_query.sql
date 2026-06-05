-- name: GetUserByEmailNoIdentity :one
SELECT u.id, u.email, u.name
FROM app."user" u
LEFT JOIN app.user_identity ui ON ui.user_id = u.id
WHERE LOWER(u.email) = LOWER($1) AND ui.id IS NULL
LIMIT 1;
