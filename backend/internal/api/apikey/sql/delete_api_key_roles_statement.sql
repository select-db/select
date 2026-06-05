-- name: DeleteAPIKeyRoles :exec
DELETE FROM auth.api_key_to_role WHERE api_key_id = $1;
