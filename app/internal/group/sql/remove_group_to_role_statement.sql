-- name: RemoveGroupToRole :exec
DELETE FROM group_to_role WHERE id = :id;
