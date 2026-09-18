-- name: DeleteGroup :exec
DELETE FROM "group"
WHERE id = :id;
