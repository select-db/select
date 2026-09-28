-- name: ListRolesScopedToDatasource :many
-- Roles whose every live rule is on this datasource: they mean nothing once it is gone.
SELECT
  r.id
FROM
  app.role r
WHERE
  r.workspace_id = $1
  AND r.deleted_at IS NULL
  AND EXISTS (
    SELECT 1 FROM app.permission p
    WHERE p.role_id = r.id AND p.deleted_at IS NULL AND p.datasource_id = sqlc.arg(datasource_id)::text
  )
  AND NOT EXISTS (
    SELECT 1 FROM app.permission p
    WHERE p.role_id = r.id AND p.deleted_at IS NULL AND p.datasource_id IS DISTINCT FROM sqlc.arg(datasource_id)::text
  );
