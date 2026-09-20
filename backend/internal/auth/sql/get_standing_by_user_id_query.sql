-- name: GetStandingByUserID :many
-- The caller's standing: one row per workspace they belong to, whether they own
-- it, and the roles they hold in it.
--
-- Membership is the spine, so the grants come in on a LEFT JOIN: a workspace
-- the caller holds no role in still has to appear. The roles arrive aggregated
-- rather than one row per grant, which keeps it one row per workspace and lets
-- COALESCE type the column honestly (sqlc reads a LEFT JOIN's plain columns as
-- NOT NULL and the role-less row then fails to scan).
--
-- Driven from the caller's own grants, which are indexed on (user_id,
-- workspace_id), rather than from app.role, which is indexed on neither and
-- would make every authenticated request scan every tenant's roles. The UNION
-- dedups a role held both directly and through a group, and each branch matches
-- the grant row's workspace against the role's own so a grant cannot reach
-- across tenants.
SELECT
  wtu.workspace_id,
  COALESCE(w.owner_id = sqlc.arg(user_id)::uuid, false)::boolean AS is_owner,
  COALESCE(g.roles, '[]'::json)::json AS roles
FROM app.workspace_to_user wtu
  JOIN app.workspace w ON w.id = wtu.workspace_id AND w.deleted_at IS NULL
  LEFT JOIN (
    SELECT
      grant_rows.workspace_id,
      json_agg(json_build_object('id', grant_rows.role_id, 'name', grant_rows.role_name)) AS roles
    FROM (
      SELECT r.id AS role_id, r.name AS role_name, r.workspace_id
      FROM app.user_to_role utr
        JOIN app.role r
          ON r.id = utr.role_id AND r.workspace_id = utr.workspace_id AND r.deleted_at IS NULL
      WHERE utr.user_id = sqlc.arg(user_id) AND utr.deleted_at IS NULL
      UNION
      -- The group's own soft-delete must be honored: deletion is a soft delete
      -- and the FK ON DELETE CASCADE only fires on hard deletes, so a deleted
      -- group would otherwise keep granting its roles through live membership.
      SELECT r.id, r.name, r.workspace_id
      FROM app.user_to_group ug
        JOIN app."group" gp ON gp.id = ug.group_id AND gp.deleted_at IS NULL
        JOIN app.group_to_role gr ON gr.group_id = ug.group_id AND gr.deleted_at IS NULL
        JOIN app.role r
          ON r.id = gr.role_id AND r.workspace_id = gr.workspace_id AND r.deleted_at IS NULL
      WHERE ug.user_id = sqlc.arg(user_id) AND ug.deleted_at IS NULL
    ) grant_rows
    GROUP BY grant_rows.workspace_id
  ) g ON g.workspace_id = wtu.workspace_id
WHERE wtu.user_id = sqlc.arg(user_id) AND wtu.deleted_at IS NULL;
