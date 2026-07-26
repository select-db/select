-- name: UpsertPermission :exec
INSERT INTO app.permission (id, role_id, workspace_id, db_instance_id, schema_name, table_name, column_name, action, effect, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
ON CONFLICT (id) DO UPDATE SET
  db_instance_id = EXCLUDED.db_instance_id,
  schema_name = EXCLUDED.schema_name,
  table_name = EXCLUDED.table_name,
  column_name = EXCLUDED.column_name,
  action = EXCLUDED.action,
  effect = EXCLUDED.effect,
  updated_at = now(),
  deleted_at = NULL;
