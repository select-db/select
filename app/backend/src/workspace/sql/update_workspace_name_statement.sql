-- name: UpdateWorkspaceName :exec
UPDATE workspace
SET name = :name
WHERE id = :id;
