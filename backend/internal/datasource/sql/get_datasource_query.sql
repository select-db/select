-- name: GetDatasource :one
SELECT
  d.db_type,
  d.name,
  d.encrypted_dsn,
  d.encrypted_ssh,
  d.max_open_conns,
  d.max_idle_conns,
  d.conn_max_lifetime,
  d.conn_max_idle_time,
  d.cellar_id,
  w.plan
FROM
  app.datasource d
  JOIN app.workspace w ON w.id = d.workspace_id
WHERE
  d.id = $1
  AND d.workspace_id = $2
  AND w.deleted_at IS NULL;
