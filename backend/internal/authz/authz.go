package authz

import (
	"net/http"

	"backend/db/generated"
	"backend/internal/audit"
	"backend/internal/middlewares"

	core "github.com/selectDb/dialect/core"
)

// RequestPrincipal builds the audit principal for a given workspace: identity
// (user or api-key, typed), display name, roles with names, and the permission
// entries in that workspace — all from the request context, so there is no extra
// DB cost. The workspace is explicit because not every route has a single member
// workspace (e.g. sync spans workspaces). The single place principals are
// assembled for the audit log.
func RequestPrincipal(r *http.Request, workspaceID string) audit.Principal {
	c := middlewares.GetPrincipal(r)

	ptype := audit.PrincipalUser
	if c.IsAPIKey {
		ptype = audit.PrincipalAPIKey
	}

	roles := make([]audit.Role, len(c.Roles))
	for i, ref := range c.Roles {
		roles[i] = audit.Role{ID: ref.ID, Name: ref.Name}
	}

	return audit.Principal{
		Type:        ptype,
		ID:          c.ID,
		Name:        c.Name,
		WorkspaceID: workspaceID,
		Roles:       roles,
		Permissions: EntriesForWorkspace(c.RoleIDs, workspaceID),
	}
}

func InSet(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func IsWorkspaceOwner(r *http.Request, workspaceID string) bool {
	return InSet(middlewares.GetOwnedWorkspaceIDs(r), workspaceID)
}

func ToDialectPermissions(rows []generated.AppPermission) []core.PermissionEntry {
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
			Action:       p.Action.String,
			Effect:       p.Effect.String,
		})
	}
	return out
}

// Keeps only permissions belonging to workspaceID
func CompiledForWorkspace(roleIDs []string, workspaceID string) core.CompiledPermissions {
	return core.Compile(EntriesForWorkspace(roleIDs, workspaceID))
}

func CompiledFromRequest(r *http.Request) core.CompiledPermissions {
	return CompiledForWorkspace(middlewares.GetRoleIDs(r), middlewares.MemberWorkspaceID(r))
}

// EntriesForWorkspace returns the raw permission entries (not compiled)
// that apply in this workspace. Use when you want to show the underlying
// rules to a user or LLM. For permission decisions, use
// CompiledForWorkspace; it bakes the wildcard and deny semantics in.
func EntriesForWorkspace(roleIDs []string, workspaceID string) []core.PermissionEntry {
	all := MergeForRoles(roleIDs)
	scoped := make([]generated.AppPermission, 0, len(all))
	for _, p := range all {
		if p.WorkspaceID.String() == workspaceID {
			scoped = append(scoped, p)
		}
	}
	return ToDialectPermissions(scoped)
}

// EntriesFromRequest is EntriesForWorkspace driven by request context.
func EntriesFromRequest(r *http.Request) []core.PermissionEntry {
	return EntriesForWorkspace(middlewares.GetRoleIDs(r), middlewares.MemberWorkspaceID(r))
}
