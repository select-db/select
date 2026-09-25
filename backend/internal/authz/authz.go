package authz

import (
	"net/http"

	"backend/db/generated"
	"backend/internal/audit"
	"backend/internal/middlewares"

	core "github.com/selectDb/dialect/core"
)

// RequestPrincipal assembles the audit principal from the request context (no
// extra DB cost). The workspace is explicit because sync spans workspaces.
func RequestPrincipal(r *http.Request, workspaceID string) audit.Principal {
	p := middlewares.GetPrincipal(r)
	wc, _ := p.Workspace(workspaceID) // roles the caller holds in this workspace

	ptype := audit.PrincipalUser
	if p.IsAPIKey {
		ptype = audit.PrincipalAPIKey
	}

	roles := make([]audit.Role, len(wc.Roles))
	roleIDs := make([]string, len(wc.Roles))
	for i, ref := range wc.Roles {
		roles[i] = audit.Role{ID: ref.ID, Name: ref.Name}
		roleIDs[i] = ref.ID
	}

	return audit.Principal{
		Type:        ptype,
		ID:          p.ID,
		Name:        p.Name,
		WorkspaceID: workspaceID,
		Roles:       roles,
		Permissions: workspaceEntries(roleIDs, workspaceID),
	}
}

func workspaceRoleIDs(r *http.Request, workspaceID string) []string {
	wc, _ := middlewares.GetPrincipal(r).Workspace(workspaceID)
	ids := make([]string, len(wc.Roles))
	for i, role := range wc.Roles {
		ids[i] = role.ID
	}
	return ids
}

func InSet(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func isWorkspaceOwner(r *http.Request, workspaceID string) bool {
	return InSet(middlewares.GetOwnedWorkspaceIDs(r), workspaceID)
}

func toEntries(rows []generated.AppPermission) []core.PermissionEntry {
	out := make([]core.PermissionEntry, 0, len(rows))
	for _, p := range rows {
		if p.DeletedAt.Valid {
			continue
		}
		out = append(out, core.PermissionEntry{
			DbInstanceID: p.DbInstanceID.Ptr(),
			SchemaName:   p.SchemaName.Ptr(),
			TableName:    p.TableName.Ptr(),
			ColumnName:   p.ColumnName.Ptr(),
			Action:       p.Action,
			Effect:       p.Effect,
		})
	}
	return out
}

// Entries are the caller's raw permission rules in its workspace, to show to a
// user or an LLM. Decide with Perms, which bakes in wildcards and denies.
func Entries(r *http.Request) []core.PermissionEntry {
	ws := middlewares.MemberWorkspaceID(r)
	return workspaceEntries(workspaceRoleIDs(r, ws), ws)
}

// Perms are the caller's compiled permissions in its workspace.
func Perms(r *http.Request) core.CompiledPermissions {
	ws := middlewares.MemberWorkspaceID(r)
	return WorkspacePerms(workspaceRoleIDs(r, ws), ws)
}

// WorkspacePerms compiles the rules roleIDs hold in workspaceID. A database
// nobody wrote a rule for is denied: the backend dials it with our credentials.
func WorkspacePerms(roleIDs []string, workspaceID string) core.CompiledPermissions {
	return core.Compile(workspaceEntries(roleIDs, workspaceID)).WithDenyUnmanaged()
}

func workspaceEntries(roleIDs []string, workspaceID string) []core.PermissionEntry {
	all := MergeForRoles(roleIDs)
	scoped := make([]generated.AppPermission, 0, len(all))
	for _, p := range all {
		if p.WorkspaceID.String() == workspaceID {
			scoped = append(scoped, p)
		}
	}
	return toEntries(scoped)
}
