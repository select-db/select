-- name: InsertManagedDatasource :exec
INSERT INTO
  app.datasource (id, workspace_id, db_type, name, state, size_bytes, updated_at)
VALUES
  ($1, $2, 'sqlite', $3, 'hot', $4, now());
