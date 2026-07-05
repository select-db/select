-- name: UpdateGroupName :exec
UPDATE "group"
SET name = :name
WHERE id = :id;
