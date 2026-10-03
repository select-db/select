-- name: RenameDatasource :exec
UPDATE app.datasource
SET
  name = $3,
  updated_at = now()
WHERE
  id = $1
  AND workspace_id = $2;
