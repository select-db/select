-- name: GetDatasource :one
SELECT
  db_type,
  name,
  encrypted_dsn,
  encrypted_ssh,
  max_open_conns,
  max_idle_conns,
  conn_max_lifetime,
  conn_max_idle_time
FROM
  app.datasource
WHERE
  id = $1
  AND workspace_id = $2;
