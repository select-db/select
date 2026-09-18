-- name: RemoveUserFromGroup :exec
DELETE FROM user_to_group WHERE id = :id;
