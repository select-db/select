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
// Role and group writes need nothing here. CreateJWT reads roles from the
// database when it mints, so the next token a user is issued carries the change
// already, and the one they hold now is accepted on its signature until it
// expires whatever this does.
//
// This is auth logic the schema can't express, so it stays hand-written and
// composes with the generated pure-upsert Apply, like needsTokenRefresh.
// Best-effort: a lookup or a single side effect failing must not fail the sync.
func applyCommitSideEffects(ctx context.Context, c types.Commit) {
	payload, _ := c.Payload.(map[string]any)
	switch c.TableName {
	case "permission":
		// A permission row changed the role's effective grants: drop its cached
		// compiled permissions so the next request reloads them from the DB.
		if rid, ok := affectedRoleID(ctx, c, payload); ok {
			authz.Invalidate(rid)
		}
	}
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
