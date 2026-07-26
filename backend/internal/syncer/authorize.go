package syncer

import (
	"backend/internal/authz"
	"backend/internal/syncer/gen"
	"backend/internal/syncer/types"
	"backend/internal/utils"

	core "github.com/selectDb/dialect/core"
)

func authorizeCommit(userID string, workspaceIDs []string, roleIDs []string, ownedWorkspaceIDs []string, c types.Commit) bool {
	if c.UserID != userID {
		return false
	}
	if c.WorkspaceID == "" {
		return false
	}
	// Commit must target a workspace the caller belongs to.
	if !authz.InSet(workspaceIDs, c.WorkspaceID) {
		return false
	}

	// Payload workspace_id must match the commit's workspace.
	payload, _ := c.Payload.(map[string]interface{})
	if wid := utils.MapGetString(payload, "workspace_id"); wid != "" && wid != c.WorkspaceID {
		return false
	}

	// Owner can perform any action.
	if authz.InSet(ownedWorkspaceIDs, c.WorkspaceID) {
		return true
	}

	compiled := authz.CompiledForWorkspace(roleIDs, c.WorkspaceID)

	// Per-table write requirements come from gen.RequiredActions (generated from
	// each @app.sync table's @app.api `requires` clause; see gen/authorize.go).
	// The AND-list captures the compound cases directly: group_to_role needs
	// groups.manage AND roles.manage (attaching a role to a group grants it to
	// every member), and workspace_to_user — the membership trust root — needs
	// roles.manage AND users.manage.
	for _, action := range gen.RequiredActions[c.TableName] {
		if !compiled.IsAllowed(action) {
			return false
		}
	}

	// workspace is the hand-written special (not @app.sync, so absent from
	// requiredActions): delete is owner-only — owners returned true above, so a
	// caller reaching here isn't the owner (mirrors the REST delete 403) — and
	// any other write needs settings.write.
	if c.TableName == "workspace" {
		if c.Operation == "delete" {
			return false
		}
		if !compiled.IsAllowed(core.ActionWorkspaceSettingsWrite) {
			return false
		}
	}

	return true
}
