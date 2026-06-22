package audit

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"sort"

	core "github.com/selectDb/dialect/core"
)

const (
	PrincipalUser   = "user"
	PrincipalAPIKey = "api_key"
)

// Principal is who acted and what they could do, as of the event. It is
// content-addressed: identical state hashes equal and is stored once.
type Principal struct {
	Type        string                 `json:"type"`           // PrincipalUser | PrincipalAPIKey
	ID          string                 `json:"id"`             // user id or api-key id
	Name        string                 `json:"name,omitempty"`
	WorkspaceID string                 `json:"workspace_id"` // workspace the action occurred in
	Roles       []Role                 `json:"roles"`
	Permissions []core.PermissionEntry `json:"permissions"`
}

type Role struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// canonical sorts roles/permissions so the hash is order-independent.
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

// JSON is the snapshot body stored in principal_snapshot.snapshot.
func (p Principal) JSON() []byte {
	b, _ := json.Marshal(p.canonical())
	return b
}

// Hash is the snapshot's content-addressed primary key.
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

// PrincipalResolver lets emit sites deep in a call chain (e.g. syncer) get the
// principal from the context. The workspace is supplied per event because a sync
// request can span workspaces.
type PrincipalResolver func(workspaceID string) Principal

type resolverCtxKey struct{}

func ContextWithPrincipalResolver(ctx context.Context, f PrincipalResolver) context.Context {
	return context.WithValue(ctx, resolverCtxKey{}, f)
}

// ResolvePrincipal runs the context's resolver, or returns a zero Principal.
func ResolvePrincipal(ctx context.Context, workspaceID string) Principal {
	if f, ok := ctx.Value(resolverCtxKey{}).(PrincipalResolver); ok && f != nil {
		return f(workspaceID)
	}
	return Principal{}
}
