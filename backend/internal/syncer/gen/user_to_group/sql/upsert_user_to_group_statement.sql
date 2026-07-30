-- name: UpsertUserToGroup :exec
INSERT INTO app.user_to_group (id, user_id, group_id, workspace_id, source, updated_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (id) DO UPDATE SET
  updated_at = now(),
  deleted_at = NULL;
