package middlewares

import (
	"context"
	"net/http/httptest"
	"testing"

	auth "backend/internal/auth"
)

// Regression: the sync route has no Membership() middleware, so building the
// principal there must not depend on the member workspace (MemberWorkspaceID
// panics when unset). GetPrincipal reads only comma-ok context values.
func TestGetPrincipalNoMemberWorkspaceDoesNotPanic(t *testing.T) {
	r := httptest.NewRequest("POST", "/sync/v1/sync", nil) // bare context, no Membership

	p := GetPrincipal(r) // must not panic

	if p.ID != "" || p.IsAPIKey || p.Name != "" || len(p.Workspaces) != 0 {
		t.Fatalf("expected zero principal on a bare request, got %+v", p)
	}
}

// The flat workspace getters all derive from the one nested structure.
func TestGetPrincipalDerivedGetters(t *testing.T) {
	ws := []auth.WorkspaceClaim{
		{ID: "w1", IsOwner: true, Roles: []auth.RoleRef{{ID: "r1", Name: "Admin"}}},
		{ID: "w2", Roles: []auth.RoleRef{{ID: "r2", Name: "Viewer"}}},
	}
	ctx := context.WithValue(context.Background(), workspacesKey, ws)
	ctx = context.WithValue(ctx, userIDKey, "u1")
	ctx = context.WithValue(ctx, principalNameKey, "Alice")
	r := httptest.NewRequest("GET", "/", nil).WithContext(ctx)

	p := GetPrincipal(r)
	if p.ID != "u1" || p.Name != "Alice" {
		t.Fatalf("identity wrong: %+v", p)
	}
	w1, ok := p.Workspace("w1")
	if !ok || !w1.IsOwner || len(w1.Roles) != 1 || w1.Roles[0].Name != "Admin" {
		t.Fatalf("w1 lookup wrong: %+v", w1)
	}

	ids, ok := GetWorkspaceIDs(r)
	if !ok || len(ids) != 2 {
		t.Fatalf("workspace ids wrong: %v", ids)
	}
	owned := GetOwnedWorkspaceIDs(r)
	if len(owned) != 1 || owned[0] != "w1" {
		t.Fatalf("owned wrong: %v", owned)
	}
}
