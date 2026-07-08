-- name: DeleteGroupToRoleByID :exec
; -- @no-track
DELETE FROM group_to_role WHERE id = :id;
