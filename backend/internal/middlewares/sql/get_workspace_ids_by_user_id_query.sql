-- name: GetWorkspaceIDsByUserID :many
SELECT
  wtu.workspace_id
FROM
  app.workspace_to_user wtu
  JOIN app.workspace w ON w.id = wtu.workspace_id
WHERE
  wtu.user_id = $1
  AND wtu.deleted_at IS NULL
  AND w.deleted_at IS NULL;
