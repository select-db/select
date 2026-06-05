-- name: DeleteDatasource :exec
DELETE FROM app.datasource
WHERE
  id = $1
  AND workspace_id = $2;