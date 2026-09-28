-- name: DeleteDatasourceRules :many
-- Removes every rule on a datasource, and the roles left with no rule anywhere
-- else. Returns each role that lost a rule, and whether the role went too.
WITH stripped AS (
  UPDATE app.permission rule
  SET deleted_at = now(), updated_at = now()
  WHERE
    rule.workspace_id = sqlc.arg(workspace_id)
    AND rule.datasource_id = sqlc.arg(datasource_id)::text
    AND rule.deleted_at IS NULL
  RETURNING rule.role_id
),
dropped AS (
  UPDATE app.role r
  SET deleted_at = now(), updated_at = now()
  WHERE
    r.workspace_id = sqlc.arg(workspace_id)
    AND r.deleted_at IS NULL
    AND r.id IN (SELECT stripped.role_id FROM stripped)
    AND NOT EXISTS (
      SELECT 1 FROM app.permission other
      WHERE other.role_id = r.id AND other.deleted_at IS NULL AND other.datasource_id IS DISTINCT FROM sqlc.arg(datasource_id)::text
    )
  RETURNING r.id
)
SELECT DISTINCT
  stripped.role_id,
  (dropped.id IS NOT NULL)::boolean AS dropped
FROM
  stripped
  LEFT JOIN dropped ON dropped.id = stripped.role_id;
