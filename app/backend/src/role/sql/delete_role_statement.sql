-- name: DeleteRole :exec
DELETE FROM role
WHERE id = :id;
