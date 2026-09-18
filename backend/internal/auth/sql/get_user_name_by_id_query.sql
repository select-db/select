-- name: GetUserNameByID :one
SELECT name, email FROM app."user" WHERE id = $1;
