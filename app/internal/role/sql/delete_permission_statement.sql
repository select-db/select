-- name: DeletePermission :exec
DELETE FROM permission
WHERE id = :id;
