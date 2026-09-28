-- name: DeleteDatasourcePermissions :many
UPDATE app.permission
SET
  deleted_at = now(),
  updated_at = now()
WHERE
  workspace_id = $1
  AND datasource_id = sqlc.arg(datasource_id)::text
  AND deleted_at IS NULL
RETURNING role_id;
