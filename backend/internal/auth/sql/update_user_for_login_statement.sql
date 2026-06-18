-- name: UpdateUserForLogin :exec
UPDATE app."user"
SET name = $2, github_id = $3, email = $4, avatar_url = $5
WHERE id = $1;
