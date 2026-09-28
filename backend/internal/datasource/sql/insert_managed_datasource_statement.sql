-- name: InsertManagedDatasource :exec
INSERT INTO
  app.datasource (id, workspace_id, db_type, name, cellar_id, state, size_bytes, updated_at)
VALUES
  ($1, $2, 'sqlite', $3, $4, 'hot', $5, now());
