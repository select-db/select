-- name: RemoveUserFromRole :exec
DELETE FROM user_to_role WHERE id = :id;
