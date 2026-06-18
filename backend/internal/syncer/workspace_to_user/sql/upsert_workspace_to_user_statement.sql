-- name: UpsertWorkspaceToUser :exec
INSERT INTO app.workspace_to_user (id, workspace_id, user_id, updated_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (id) DO UPDATE SET
  workspace_id = EXCLUDED.workspace_id,
  user_id = EXCLUDED.user_id,
  updated_at = now(),
  deleted_at = NULL
WHERE app.workspace_to_user.workspace_id = EXCLUDED.workspace_id;
