-- name: UpdateUserEmail :exec
UPDATE app."user"
SET email = $2
WHERE id = $1;
