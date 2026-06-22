package middlewares

import (
	"net/http/httptest"
	"testing"
)

// Regression: the sync route has no Membership() middleware, so building the
// principal there must not depend on the member workspace (MemberWorkspaceID
// panics when unset). GetPrincipal reads only comma-ok context values.
func TestGetPrincipalNoMemberWorkspaceDoesNotPanic(t *testing.T) {
	r := httptest.NewRequest("POST", "/sync/v1/sync", nil) // bare context, no Membership

	p := GetPrincipal(r) // must not panic

	if p.ID != "" || p.IsAPIKey || p.Name != "" || len(p.RoleIDs) != 0 || len(p.Roles) != 0 {
		t.Fatalf("expected zero principal on a bare request, got %+v", p)
	}
}
