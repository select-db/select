-- name: GetWorkspaceMembershipsByUserID :many
-- The workspaces the caller belongs to, and whether they own each one. The
-- spine of their standing: a workspace they hold no role in still belongs here.
SELECT
  wtu.workspace_id,
  COALESCE(w.owner_id = sqlc.arg(user_id)::uuid, false)::boolean AS is_owner
FROM app.workspace_to_user wtu
  JOIN app.workspace w ON w.id = wtu.workspace_id AND w.deleted_at IS NULL
WHERE wtu.user_id = sqlc.arg(user_id) AND wtu.deleted_at IS NULL;
