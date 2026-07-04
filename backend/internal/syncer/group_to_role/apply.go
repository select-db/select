package group_to_role

import (
	"context"
	"fmt"
	"time"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/audit"
	"backend/internal/syncer/patch"
	"backend/internal/syncer/scope"
	"backend/internal/syncer/types"
	"backend/internal/utils"
)

func Apply(ctx context.Context, userID string, c types.Commit, lastPulledAt time.Time) (bool, *types.RestoredItem, error) {
	payload, ok := c.Payload.(map[string]any)
	if !ok {
		return false, nil, fmt.Errorf("group_to_role: payload must be an object")
	}

	id := utils.MapGetString(payload, "id")
	workspaceID := c.WorkspaceID
	groupID := utils.MapGetString(payload, "group_id")
	roleID := utils.MapGetString(payload, "role_id")
	if id == "" || workspaceID == "" || groupID == "" || roleID == "" {
		return false, nil, fmt.Errorf("group_to_role: missing id, workspace_id, group_id or role_id")
	}

	idUUID, err := db_types.NewJSONNullUUIDFromString(id)
	if err != nil {
		return false, nil, fmt.Errorf("group_to_role: invalid id %q: %w", id, err)
	}
	workspaceUUID, err := db_types.NewJSONNullUUIDFromString(workspaceID)
	if err != nil {
		return false, nil, fmt.Errorf("group_to_role: invalid workspace_id %q: %w", workspaceID, err)
	}
	groupUUID, err := db_types.NewJSONNullUUIDFromString(groupID)
	if err != nil {
		return false, nil, fmt.Errorf("group_to_role: invalid group_id %q: %w", groupID, err)
	}
	roleUUID, err := db_types.NewJSONNullUUIDFromString(roleID)
	if err != nil {
		return false, nil, fmt.Errorf("group_to_role: invalid role_id %q: %w", roleID, err)
	}

	// Both sides must live in this workspace, else a groups.manage holder could
	// point a group at a role in another workspace (privilege escalation).
	if ok, err := scope.GroupInWorkspace(ctx, groupUUID, workspaceUUID); err != nil {
		return false, nil, fmt.Errorf("group_to_role: validate group: %w", err)
	} else if !ok {
		return false, nil, fmt.Errorf("group_to_role: group_id does not belong to workspace")
	}
	if ok, err := scope.RoleInWorkspace(ctx, roleUUID, workspaceUUID); err != nil {
		return false, nil, fmt.Errorf("group_to_role: validate role: %w", err)
	} else if !ok {
		return false, nil, fmt.Errorf("group_to_role: role_id does not belong to workspace")
	}

	return patch.Apply(ctx, c, patch.Handler[generated.AppGroupToRole, generated.UpsertGroupToRoleParams]{
		TableName: "group_to_role",
		Audit:     &patch.AuditConfig{Spec: audit.GroupRoleAttached, TargetID: groupID},
		Fetch: func(ctx context.Context) (generated.AppGroupToRole, error) {
			return db.Queries.GetGroupToRoleByID(ctx, generated.GetGroupToRoleByIDParams{
				ID:          idUUID,
				WorkspaceID: workspaceUUID,
			})
		},
		UpdatedAt: func(row generated.AppGroupToRole) time.Time {
			return row.UpdatedAt.ValueOrZero()
		},
		DeletedAt: func(row generated.AppGroupToRole) *time.Time {
			if row.DeletedAt.Valid {
				t := row.DeletedAt.ValueOrZero()
				return &t
			}
			return nil
		},
		Restored: func(row generated.AppGroupToRole) (interface{}, error) {
			return types.ToRestoredPayload(appGroupToRoleToTypesRow(row))
		},
		Merge: func(existing generated.AppGroupToRole, isNew bool, payload map[string]any) (generated.UpsertGroupToRoleParams, error) {
			gid := groupUUID
			rid := roleUUID
			wid := workspaceUUID
			if !isNew {
				gid = utils.PatchUUID(payload, "group_id", existing.GroupID, groupUUID)
				rid = utils.PatchUUID(payload, "role_id", existing.RoleID, roleUUID)
				wid = utils.PatchUUID(payload, "workspace_id", existing.WorkspaceID, workspaceUUID)
			}
			return generated.UpsertGroupToRoleParams{
				ID:          idUUID,
				GroupID:     gid,
				RoleID:      rid,
				WorkspaceID: wid,
			}, nil
		},
		Upsert: func(ctx context.Context, params generated.UpsertGroupToRoleParams) error {
			if err := db.Queries.UpsertGroupToRole(ctx, params); err != nil {
				return err
			}
			// A group's role set changed: every member's effective roles (baked into
			// their token) shift, so revoke refresh tokens for all current members.
			revokeGroupMembers(ctx, groupUUID)
			return nil
		},
	})
}

// revokeGroupMembers forces a token refresh for every active member of a group,
// so a group->role change takes effect on their next request. Best-effort:
// membership lookup or a single revocation failing must not fail the sync.
func revokeGroupMembers(ctx context.Context, groupID db_types.JSONNullUUID) {
	members, err := db.Queries.GetUserIDsByGroupID(ctx, groupID)
	if err != nil {
		return
	}
	for _, m := range members {
		_ = db.Queries.DeleteUserRefreshTokens(ctx, m)
	}
}
