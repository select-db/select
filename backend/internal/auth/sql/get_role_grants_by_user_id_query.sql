-- name: GetRoleGrantsByUserID :many
-- Every role the caller holds, directly or through a group, with the workspace
-- it belongs to.
--
-- Driven from the caller's own grants, which are indexed on (user_id,
-- workspace_id), rather than from app.role, which is indexed on neither and
-- would make every authenticated request scan every tenant's roles. The UNION
-- is also what dedups a role held both ways.
SELECT r.id AS role_id, r.name AS role_name, r.workspace_id
FROM app.user_to_role utr
  JOIN app.role r ON r.id = utr.role_id AND r.deleted_at IS NULL
WHERE utr.user_id = sqlc.arg(user_id) AND utr.deleted_at IS NULL
UNION
-- The group's own soft-delete must be honored: deletion is a soft delete and the
-- FK ON DELETE CASCADE only fires on hard deletes, so a deleted group would
-- otherwise keep granting its roles through live membership rows.
SELECT r.id, r.name, r.workspace_id
FROM app.user_to_group ug
  JOIN app."group" g ON g.id = ug.group_id AND g.deleted_at IS NULL
  JOIN app.group_to_role gr ON gr.group_id = ug.group_id AND gr.deleted_at IS NULL
  JOIN app.role r ON r.id = gr.role_id AND r.deleted_at IS NULL
WHERE ug.user_id = sqlc.arg(user_id) AND ug.deleted_at IS NULL;
