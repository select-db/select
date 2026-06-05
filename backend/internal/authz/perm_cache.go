package authz

import (
	"context"
	"time"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"

	"github.com/selectDb/toolkit/cache"
)

// ~310 B/perm × 20 perms/role ≈ 6 KB/role → 15 000 roles ≈ 90 MB worst-case.
var store = cache.New(cache.Options{
	MaxEntries: 15_000,
	TTL:        2 * time.Hour,
})

// Invalidate drops cached permissions for a role, forcing a DB reload on next request.
func Invalidate(roleID string) {
	store.Delete(roleID)
}

// GetOrLoad returns cached permissions for a role, loading from DB on cache miss.
func GetOrLoad(roleID string) ([]generated.AppPermission, error) {
	if v, ok := store.Get(roleID); ok {
		return v.([]generated.AppPermission), nil
	}

	uid, err := db_types.NewJSONNullUUIDFromString(roleID)
	if err != nil {
		return nil, err
	}
	rows, err := db.Queries.GetPermissionsByRoleID(context.Background(), uid)
	if err != nil {
		return nil, err
	}

	perms := make([]generated.AppPermission, len(rows))
	for i, r := range rows {
		perms[i] = generated.AppPermission{
			ID:           r.ID,
			RoleID:       r.RoleID,
			WorkspaceID:  r.WorkspaceID,
			DbInstanceID: r.DbInstanceID,
			SchemaName:   r.SchemaName,
			TableName:    r.TableName,
			ColumnName:   r.ColumnName,
			Action:       r.Action,
			Effect:       r.Effect,
			UpdatedAt:    r.UpdatedAt,
			DeletedAt:    r.DeletedAt,
		}
	}

	store.Set(roleID, perms)
	return perms, nil
}

// MergeForRoles returns all active permissions across the given roleIDs.
func MergeForRoles(roleIDs []string) []generated.AppPermission {
	var out []generated.AppPermission
	for _, id := range roleIDs {
		perms, _ := GetOrLoad(id)
		out = append(out, perms...)
	}
	return out
}
