-- name: UpsertGroupToRole :exec
INSERT INTO app.group_to_role (id, group_id, role_id, workspace_id, updated_at)
VALUES ($1, $2, $3, $4, now())
ON CONFLICT (id) DO UPDATE SET
  group_id = EXCLUDED.group_id,
  role_id = EXCLUDED.role_id,
  workspace_id = EXCLUDED.workspace_id,
  updated_at = now(),
  deleted_at = NULL;
