-- name: ListDatasourcesByWorkspace :many
SELECT
  d.id,
  d.db_type,
  d.name
FROM
  app.datasource d
  JOIN app.workspace w ON w.id = d.workspace_id
WHERE
  d.workspace_id = $1
  AND w.deleted_at IS NULL
  AND d.state IS DISTINCT FROM 'deleting'
ORDER BY
  d.name,
  d.id;
