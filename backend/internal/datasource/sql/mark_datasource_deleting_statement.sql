-- name: MarkDatasourceDeleting :exec
UPDATE app.datasource
SET
  state = 'deleting',
  updated_at = now()
WHERE
  id = $1
  AND workspace_id = $2
  AND cellar_id IS NOT NULL;
