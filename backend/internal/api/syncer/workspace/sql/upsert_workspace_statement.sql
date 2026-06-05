-- name: UpsertWorkspace :exec
INSERT INTO app.workspace (id, name, git_remote_url, owner_id, updated_at)
VALUES ($1, $2, $3, $4, now())
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  git_remote_url = EXCLUDED.git_remote_url,
  owner_id = COALESCE(EXCLUDED.owner_id, app.workspace.owner_id),
  updated_at = now(),
  deleted_at = NULL;
