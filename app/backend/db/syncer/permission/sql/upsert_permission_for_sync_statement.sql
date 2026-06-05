-- name: UpsertPermissionForSync :exec
; -- @no-track
INSERT INTO permission (id, role_id, workspace_id, db_instance_id, schema_name, table_name, column_name, action, effect)
VALUES (:id, :role_id, :workspace_id, :db_instance_id, :schema_name, :table_name, :column_name, :action, :effect)
ON CONFLICT (id) DO UPDATE SET
    role_id = excluded.role_id,
    workspace_id = excluded.workspace_id,
    db_instance_id = excluded.db_instance_id,
    schema_name = excluded.schema_name,
    table_name = excluded.table_name,
    column_name = excluded.column_name,
    action = excluded.action,
    effect = excluded.effect;
