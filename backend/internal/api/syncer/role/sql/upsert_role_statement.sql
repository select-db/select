-- name: UpsertRole :exec
INSERT INTO app.role (id, workspace_id, name, updated_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  updated_at = now(),
  deleted_at = NULL;
