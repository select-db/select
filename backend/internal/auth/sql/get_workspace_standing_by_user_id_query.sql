-- name: GetWorkspaceStandingByUserID :many
-- The caller's standing in every workspace they are a member of: whether they
-- own it, and the roles they hold there, directly or through a group. One row
-- per (workspace, role), and one role-less row for a workspace they hold no
-- role in, so membership is the spine rather than the roles.
--
-- A role granted both directly and through a group is one row, not two: the
-- grants are an OR over the same role rather than a join per path.
SELECT
  wtu.workspace_id,
  COALESCE(w.owner_id = sqlc.arg(user_id)::uuid, false)::boolean AS is_owner,
  r.id AS role_id,
  r.name AS role_name
FROM app.workspace_to_user wtu
  JOIN app.workspace w ON w.id = wtu.workspace_id AND w.deleted_at IS NULL
  LEFT JOIN app.role r
    ON r.workspace_id = wtu.workspace_id
    AND r.deleted_at IS NULL
    AND (
      EXISTS (
        SELECT 1 FROM app.user_to_role utr
        WHERE utr.role_id = r.id AND utr.user_id = sqlc.arg(user_id) AND utr.deleted_at IS NULL
      )
      OR EXISTS (
        -- The group's own soft-delete must be honored: deletion is a soft delete
        -- and the FK ON DELETE CASCADE only fires on hard deletes, so a deleted
        -- group would otherwise keep granting its roles through live membership.
        SELECT 1 FROM app.group_to_role gr
          JOIN app.user_to_group ug ON ug.group_id = gr.group_id AND ug.deleted_at IS NULL
          JOIN app."group" g ON g.id = gr.group_id AND g.deleted_at IS NULL
        WHERE gr.role_id = r.id AND gr.deleted_at IS NULL AND ug.user_id = sqlc.arg(user_id)
      )
    )
WHERE wtu.user_id = sqlc.arg(user_id) AND wtu.deleted_at IS NULL;
