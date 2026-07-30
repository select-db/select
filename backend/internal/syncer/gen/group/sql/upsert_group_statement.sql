-- name: UpsertGroup :exec
INSERT INTO app.group (id, workspace_id, name, source, external_id, updated_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  source = EXCLUDED.source,
  external_id = EXCLUDED.external_id,
  updated_at = now(),
  deleted_at = NULL;
