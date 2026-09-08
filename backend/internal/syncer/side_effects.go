package syncer

import (
	"context"

	"backend/db"
	"backend/db/generated"
	"backend/internal/authz"
	"backend/internal/syncer/types"
	"backend/internal/utils"

	"github.com/google/uuid"
)

// applyCommitSideEffects runs the hand-written domain side effects a synced
// write triggers beyond the row upsert itself: revoking refresh tokens for
// users whose effective roles shift, and dropping cached compiled permissions
// so an authz change takes effect on the next request instead of lingering
// behind the current JWT or the permission cache TTL. It runs after a
// successful apply.
//
// This is auth logic the schema can't express — which users a write affects,
// whether it fans out to a whole group, which cache it dirties — so it stays
// hand-written and composes with the generated pure-upsert Apply, like
// needsTokenRefresh. Best-effort: a lookup or a single side effect failing must
// not fail the sync (mirrors the pre-generation hand-written behaviour).
func applyCommitSideEffects(ctx context.Context, c types.Commit) {
	payload, _ := c.Payload.(map[string]any)
	switch c.TableName {
	case "user_to_role", "user_to_group":
		// A single user's membership changed: revoke just that user.
		if uid, ok := affectedUserID(ctx, c, payload); ok {
			_ = db.Queries.DeleteUserRefreshTokens(ctx, uid)
		}
	case "group_to_role":
		// A group's role set changed: every current member is affected.
		if gid, ok := affectedGroupID(ctx, c, payload); ok {
			revokeGroupMembers(ctx, gid)
		}
	case "group":
		// Deleting a group removes its members' group-derived roles.
		if c.Operation == "delete" {
			if gid, err := uuid.Parse(c.ObjectID); err == nil {
				revokeGroupMembers(ctx, gid)
			}
		}
	case "permission":
		// A permission row changed the role's effective grants: drop its cached
		// compiled permissions so the next request reloads them from the DB.
		if rid, ok := affectedRoleID(ctx, c, payload); ok {
			authz.Invalidate(rid)
		}
	}
}

// affectedUserID resolves the user a user_to_role / user_to_group commit
// touches: from the payload on insert, or by fetching the row on delete (whose
// payload carries only id + workspace_id). The row survives the soft-delete, so
// the post-apply fetch still finds it.
func affectedUserID(ctx context.Context, c types.Commit, payload map[string]any) (uuid.UUID, bool) {
	if c.Operation != "delete" {
		uid, err := uuid.Parse(utils.MapGetString(payload, "user_id"))
		return uid, err == nil
	}
	id, ws, ok := commitRowKey(c, payload)
	if !ok {
		return uuid.UUID{}, false
	}
	switch c.TableName {
	case "user_to_role":
		row, err := db.Queries.GetUserToRoleByID(ctx, generated.GetUserToRoleByIDParams{ID: id, WorkspaceID: ws})
		return row.UserID, err == nil
	case "user_to_group":
		row, err := db.Queries.GetUserToGroupByID(ctx, generated.GetUserToGroupByIDParams{ID: id, WorkspaceID: ws})
		return row.UserID, err == nil
	}
	return uuid.UUID{}, false
}

// affectedGroupID resolves the group a group_to_role commit touches: from the
// payload on insert, or by fetching the row on delete.
func affectedGroupID(ctx context.Context, c types.Commit, payload map[string]any) (uuid.UUID, bool) {
	if c.Operation != "delete" {
		gid, err := uuid.Parse(utils.MapGetString(payload, "group_id"))
		return gid, err == nil
	}
	id, ws, ok := commitRowKey(c, payload)
	if !ok {
		return uuid.UUID{}, false
	}
	row, err := db.Queries.GetGroupToRoleByID(ctx, generated.GetGroupToRoleByIDParams{ID: id, WorkspaceID: ws})
	return row.GroupID, err == nil
}

// affectedRoleID resolves the role a permission commit touches: from the
// payload on insert, or by fetching the row on delete. Returns the role's UUID
// string, which is the authz permission-cache key.
func affectedRoleID(ctx context.Context, c types.Commit, payload map[string]any) (string, bool) {
	if c.Operation != "delete" {
		rid := utils.MapGetString(payload, "role_id")
		return rid, rid != ""
	}
	id, ws, ok := commitRowKey(c, payload)
	if !ok {
		return "", false
	}
	row, err := db.Queries.GetPermissionByID(ctx, generated.GetPermissionByIDParams{ID: id, WorkspaceID: ws})
	if err != nil {
		return "", false
	}
	return row.RoleID.String(), true
}

// commitRowKey parses the (id, workspace_id) that identify the commit's row,
// taking id from the payload with a fallback to ObjectID (matching ApplyDelete).
func commitRowKey(c types.Commit, payload map[string]any) (id, ws uuid.UUID, ok bool) {
	rawID := utils.MapGetString(payload, "id")
	if rawID == "" {
		rawID = c.ObjectID
	}
	id, err1 := uuid.Parse(rawID)
	ws, err2 := uuid.Parse(c.WorkspaceID)
	return id, ws, err1 == nil && err2 == nil
}

// revokeGroupMembers revokes refresh tokens for every current member of a
// group. Best-effort, per the file-level contract.
func revokeGroupMembers(ctx context.Context, groupID uuid.UUID) {
	members, err := db.Queries.GetUserIDsByGroupID(ctx, groupID)
	if err != nil {
		return
	}
	for _, m := range members {
		_ = db.Queries.DeleteUserRefreshTokens(ctx, m)
	}
}
