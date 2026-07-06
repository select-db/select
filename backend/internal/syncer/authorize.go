package syncer

import (
	"backend/internal/authz"
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

	// Membership and RBAC tables require manage authority (owner already
	// returned above). workspace_to_user is included: it is the membership
	// trust root every other check reads, so it must not be member-writable.
	manageTables := map[string]struct{}{
		"role":              {},
		"user_to_role":      {},
		"permission":        {},
		"workspace_to_user": {},
	}
	_, isManageOperation := manageTables[c.TableName]
	if isManageOperation && !compiled.IsAllowed(core.ActionWorkspaceRolesManage) {
		return false
	}

	// Group tables are gated by their own action, symmetric to roles.manage:
	// managing the identity layer is a distinct privilege from managing roles.
	groupTables := map[string]struct{}{
		"group":         {},
		"user_to_group": {},
		"group_to_role": {},
	}
	if _, ok := groupTables[c.TableName]; ok && !compiled.IsAllowed(core.ActionWorkspaceGroupsManage) {
		return false
	}

	// Attaching a role to a group grants that role to every member, so it also
	// requires roles.manage — symmetric to assigning a role directly to a user
	// (user_to_role, gated by roles.manage above). This prevents a groups.manage
	// holder from handing out roles they couldn't otherwise grant.
	if c.TableName == "group_to_role" && !compiled.IsAllowed(core.ActionWorkspaceRolesManage) {
		return false
	}

	// Workspace delete is owner-only; owners returned true above, so a caller
	// here is not the owner (mirrors the REST delete handler's 403)
	if c.TableName == "workspace" && c.Operation == "delete" {
		return false
	}

	if c.TableName == "workspace" && !compiled.IsAllowed(core.ActionWorkspaceSettingsWrite) {
		return false
	}

	if c.TableName == "workspace_to_user" && !compiled.IsAllowed(core.ActionWorkspaceUsersManage) {
		return false
	}

	return true
}
