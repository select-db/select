-- name: UpsertDatasource :exec
INSERT INTO
  app.datasource (
    id,
    workspace_id,
    db_type,
    name,
    encrypted_dsn,
    encrypted_ssh,
    max_open_conns,
    max_idle_conns,
    conn_max_lifetime,
    conn_max_idle_time,
    updated_at
  )
VALUES
  (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10,
    now()
  )
ON CONFLICT (id) DO UPDATE
SET
  db_type = EXCLUDED.db_type,
  name = EXCLUDED.name,
  encrypted_dsn = EXCLUDED.encrypted_dsn,
  encrypted_ssh = EXCLUDED.encrypted_ssh,
  max_open_conns = EXCLUDED.max_open_conns,
  max_idle_conns = EXCLUDED.max_idle_conns,
  conn_max_lifetime = EXCLUDED.conn_max_lifetime,
  conn_max_idle_time = EXCLUDED.conn_max_idle_time,
  updated_at = now();
