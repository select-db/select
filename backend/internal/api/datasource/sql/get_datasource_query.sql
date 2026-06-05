-- name: GetDatasource :one
SELECT
  db_type,
  name,
  pgp_sym_decrypt (encrypted_dsn, $2)::text AS dsn,
  pgp_sym_decrypt (encrypted_ssh, $2)::text AS ssh,
  max_open_conns,
  max_idle_conns,
  conn_max_lifetime,
  conn_max_idle_time
FROM
  app.datasource
WHERE
  id = $1
  AND workspace_id = $3;
