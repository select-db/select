-- name: UpdateRoleName :exec
UPDATE role
SET name = :name
WHERE id = :id;
