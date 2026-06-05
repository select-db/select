package authz

import (
	"net/http"

	"backend/db/generated"
	"backend/internal/middlewares"

	core "github.com/selectDb/dialect/core"
)

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
