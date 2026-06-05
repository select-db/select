-- name: GetWorkspaceByID :one
SELECT id, name, git_remote_url, last_pulled_at
FROM workspace
WHERE id = :id;
