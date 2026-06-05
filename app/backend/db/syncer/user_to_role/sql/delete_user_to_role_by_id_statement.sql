-- name: DeleteUserToRoleByID :exec
; -- @no-track
DELETE FROM user_to_role WHERE id = :id;
