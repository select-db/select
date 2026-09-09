-- name: GetWorkspaceLocalPath :one
SELECT local_path
FROM workspace
WHERE id = :id;
