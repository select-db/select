-- name: UpsertWorkspaceToUser :exec
INSERT INTO app.workspace_to_user (id, workspace_id, user_id, updated_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (id) DO UPDATE SET
  updated_at = now(),
  deleted_at = NULL;
