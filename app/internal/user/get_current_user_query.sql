-- name: GetCurrentUser :one
SELECT
    u.*
FROM
    user u
WHERE
    u.current = TRUE
LIMIT 1;
