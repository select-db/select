-- name: ClearWorkspaceDatasourceSecrets :exec
UPDATE app.datasource
SET
  encrypted_dsn = NULL,
  encrypted_ssh = NULL,
  updated_at = NOW()
WHERE
  workspace_id = $1
  AND (encrypted_dsn IS NOT NULL OR encrypted_ssh IS NOT NULL);
