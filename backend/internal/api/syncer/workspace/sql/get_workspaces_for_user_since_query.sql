-- name: GetWorkspacesForUserSince :many
SELECT w.id, w.name, w.git_remote_url, w.owner_id, w.updated_at, w.deleted_at
FROM app.workspace w
INNER JOIN app.workspace_to_user wtu ON wtu.workspace_id = w.id AND wtu.user_id = $1
WHERE w.updated_at > $2;
