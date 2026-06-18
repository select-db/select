-- name: ListMyPermissions :many
SELECT
  p.id,
  p.role_id,
  p.workspace_id,
  p.db_instance_id,
  p.schema_name,
  p.table_name,
  p.column_name,
  p.action,
  p.effect,
  r.name role_name
FROM
  permission p
  LEFT JOIN user_to_role utr ON utr.role_id = p.role_id
  LEFT JOIN role r ON r.id = utr.role_id
WHERE
  utr.user_id =:user_id
  AND utr.workspace_id =:workspace_id
  AND utr.deleted_at IS NULL
  AND p.deleted_at IS NULL
ORDER BY
  p.db_instance_id,
  p.schema_name,
  p.table_name,
  p.column_name,
  p.action;