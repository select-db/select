-- name: UpsertUserToRole :exec
INSERT INTO app.user_to_role (id, user_id, role_id, workspace_id, updated_at)
VALUES ($1, $2, $3, $4, now())
ON CONFLICT (id) DO UPDATE SET
  user_id = EXCLUDED.user_id,
  role_id = EXCLUDED.role_id,
  workspace_id = EXCLUDED.workspace_id,
  updated_at = now(),
  deleted_at = NULL;
