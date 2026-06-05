-- name: InsertPermission :one
INSERT INTO permission (id, role_id, workspace_id, db_instance_id, schema_name, table_name, column_name, action, effect)
VALUES (:id, :role_id, :workspace_id, :db_instance_id, :schema_name, :table_name, :column_name, :action, :effect)
ON CONFLICT(role_id, COALESCE(db_instance_id, ''), COALESCE(schema_name, ''), COALESCE(table_name, ''), COALESCE(column_name, ''), action) DO UPDATE SET effect = excluded.effect
RETURNING id, role_id, workspace_id, db_instance_id, schema_name, table_name, column_name, action, effect;
