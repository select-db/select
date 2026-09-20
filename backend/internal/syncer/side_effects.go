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
// write triggers beyond the row upsert itself. It runs after a successful
// apply.
//
// Role and group writes need nothing here: standing is derived per request, so
// the write is the whole of the change.
//
// This is auth logic the schema can't express, so it stays hand-written and
// composes with the generated pure-upsert Apply. Best-effort: a lookup failing
// must not fail the sync.
func applyCommitSideEffects(ctx context.Context, c types.Commit) {
	if c.TableName != "permission" {
		return
	}
	// The row changed the role's effective grants, so drop its cached compiled
	// permissions and let the next request reload them from the DB.
	payload, _ := c.Payload.(map[string]any)
	if roleID, ok := affectedRoleID(ctx, c, payload); ok {
		authz.Invalidate(roleID)
	}
}

// affectedRoleID resolves the role a permission commit touches: from the
// payload on insert, or by fetching the row on delete. Returns the role's UUID
// string, which is the authz permission-cache key.
func affectedRoleID(ctx context.Context, c types.Commit, payload map[string]any) (string, bool) {
	if c.Operation != "delete" {
		roleID := utils.MapGetString(payload, "role_id")
		return roleID, roleID != ""
	}
	// The payload of a delete carries only id + workspace_id, and id falls back
	// to ObjectID the way ApplyDelete reads it.
	rawID := utils.MapGetString(payload, "id")
	if rawID == "" {
		rawID = c.ObjectID
	}
	id, err := uuid.Parse(rawID)
	if err != nil {
		return "", false
	}
	workspaceID, err := uuid.Parse(c.WorkspaceID)
	if err != nil {
		return "", false
	}
	row, err := db.Queries.GetPermissionByID(ctx, generated.GetPermissionByIDParams{ID: id, WorkspaceID: workspaceID})
	if err != nil {
		return "", false
	}
	return row.RoleID.String(), true
}
