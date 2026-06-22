package audit

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"sort"

	core "github.com/selectDb/dialect/core"
)

// Principal kinds.
const (
	PrincipalUser   = "user"
	PrincipalAPIKey = "api_key"
)

// Principal is the snapshot of who acted and what they were allowed to do, as
// of the event. It is content-addressed: identical authz state hashes to the
// same value and is stored once in audit.principal_snapshot.
type Principal struct {
	Type        string                 `json:"type"`           // PrincipalUser | PrincipalAPIKey
	ID          string                 `json:"id"`             // user id or api-key id
	Name        string                 `json:"name,omitempty"` // display name (user name/email, or key name)
	WorkspaceID string                 `json:"workspace_id"`   // workspace the action occurred in
	Roles       []Role                 `json:"roles"`          // roles with names, as of the event
	Permissions []core.PermissionEntry `json:"permissions"`    // raw entries at event time
}

// Role is a role id with its human-readable name, captured in the snapshot.
type Role struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// canonical returns a deterministically-ordered copy so the hash is stable
// regardless of the order roles/permissions arrive in.
func (p Principal) canonical() Principal {
	roles := append([]Role(nil), p.Roles...)
	sort.Slice(roles, func(i, j int) bool { return roles[i].ID < roles[j].ID })

	perms := append([]core.PermissionEntry(nil), p.Permissions...)
	sort.Slice(perms, func(i, j int) bool { return permKey(perms[i]) < permKey(perms[j]) })

	return Principal{
		Type:        p.Type,
		ID:          p.ID,
		Name:        p.Name,
		WorkspaceID: p.WorkspaceID,
		Roles:       roles,
		Permissions: perms,
	}
}

// JSON is the canonical snapshot body stored in principal_snapshot.snapshot.
func (p Principal) JSON() []byte {
	b, _ := json.Marshal(p.canonical())
	return b
}

// Hash is the content address (primary key) of the snapshot.
func (p Principal) Hash() []byte {
	sum := sha256.Sum256(p.JSON())
	return sum[:]
}

func permKey(e core.PermissionEntry) string {
	s := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	return s(e.DbInstanceID) + "|" + s(e.SchemaName) + "|" + s(e.TableName) + "|" +
		s(e.ColumnName) + "|" + e.Action + "|" + e.Effect + "|" + e.RoleName
}

// PrincipalResolver builds the audit principal for a given workspace. Emit sites
// deep in a call chain (e.g. syncer apply paths) get it from the context rather
// than threading it through every signature; the workspace is supplied per event
// (a sync request can span workspaces), so permissions are resolved correctly.
type PrincipalResolver func(workspaceID string) Principal

type resolverCtxKey struct{}

// ContextWithPrincipalResolver stashes the resolver for downstream emit sites.
func ContextWithPrincipalResolver(ctx context.Context, f PrincipalResolver) context.Context {
	return context.WithValue(ctx, resolverCtxKey{}, f)
}

// ResolvePrincipal builds the principal for workspaceID using the resolver set
// by ContextWithPrincipalResolver, or a zero Principal if none is set.
func ResolvePrincipal(ctx context.Context, workspaceID string) Principal {
	if f, ok := ctx.Value(resolverCtxKey{}).(PrincipalResolver); ok && f != nil {
		return f(workspaceID)
	}
	return Principal{}
}
