-- name: ListDatasourcesByWorkspace :many
SELECT
  id,
  db_type,
  name
FROM
  app.datasource
WHERE
  workspace_id = $1
ORDER BY
  name,
  id;
