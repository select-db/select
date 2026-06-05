-- name: InsertUserPlaceholder :one
INSERT INTO app."user" (email, name)
VALUES ($1, $2)
RETURNING id;