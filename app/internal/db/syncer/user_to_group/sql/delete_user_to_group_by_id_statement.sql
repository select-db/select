-- name: DeleteUserToGroupByID :exec
; -- @no-track
DELETE FROM user_to_group WHERE id = :id;
