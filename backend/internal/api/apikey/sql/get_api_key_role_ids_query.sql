-- name: GetAPIKeyRoleIDs :many
SELECT role_id FROM auth.api_key_to_role
WHERE api_key_id = $1;
