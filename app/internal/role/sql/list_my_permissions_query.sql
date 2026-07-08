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
FROM permission p
  JOIN role r ON r.id = p.role_id AND r.deleted_at IS NULL
WHERE p.deleted_at IS NULL
  AND p.workspace_id = :workspace_id
  -- A user's effective roles: assigned directly (user_to_role) or granted through
  -- a group they belong to (user_to_group -> group_to_role). The "group" join drops
  -- roles from a locally-deleted group whose membership rows still linger.
  AND p.role_id IN (
    SELECT utr.role_id FROM user_to_role utr
    WHERE utr.user_id = :user_id AND utr.deleted_at IS NULL
    UNION
    SELECT gr.role_id FROM group_to_role gr
      JOIN user_to_group ug ON ug.group_id = gr.group_id AND ug.deleted_at IS NULL
      JOIN "group" g ON g.id = gr.group_id AND g.deleted_at IS NULL
    WHERE ug.user_id = :user_id AND gr.deleted_at IS NULL
  )
ORDER BY
  p.db_instance_id,
  p.schema_name,
  p.table_name,
  p.column_name,
  p.action;
