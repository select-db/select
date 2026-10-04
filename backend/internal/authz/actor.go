package authz

import (
	"net/http"

	"backend/internal/middlewares"

	"github.com/selectDb/dialect/core"
)

// Actor is a request's identity plus its authorization, read in one place so a
// handler doesn't reassemble it from the middleware/authz grab-bag. Its methods
// answer the authorization questions a handler asks inline. Valid only behind the
// Authenticated middleware (and Membership, for WorkspaceID).
type Actor struct {
	UserID      string
	WorkspaceID string // the request's target workspace (from Membership)
	IsAPIKey    bool

	r *http.Request
}

// ActorOf reads the caller's identity and target workspace from the request.
func ActorOf(r *http.Request) Actor {
	p := middlewares.GetPrincipal(r)
	return Actor{
		UserID:      p.ID,
		WorkspaceID: middlewares.MemberWorkspaceID(r),
		IsAPIKey:    p.IsAPIKey,
		r:           r,
	}
}

// IsOwner reports whether the actor owns its target workspace.
func (a Actor) IsOwner() bool { return isWorkspaceOwner(a.r, a.WorkspaceID) }

// Can reports whether the actor holds a workspace-level permission, e.g.
// core.ActionWorkspaceApiKeysManage.
func (a Actor) Can(action string) bool { return Perms(a.r).IsAllowed(action) }

// ManagesDatasources reports whether the actor administers every datasource of
// its workspace: it owns the workspace, or holds workspace/datasources.manage.
// Adding a datasource asks this.
func (a Actor) ManagesDatasources() bool {
	return a.IsOwner() || a.Can(core.ActionWorkspaceDatasourcesManage)
}

// ManagesDatasource reports whether the actor may administer one datasource:
// fork, download, change or delete it. That is ManagesDatasources, or manage on
// that datasource alone.
func (a Actor) ManagesDatasource(datasourceID string) bool {
	if a.IsOwner() {
		return true
	}
	// Compiled once for both questions: Perms compiles the actor's rules.
	perms := Perms(a.r)
	return perms.IsAllowed(core.ActionWorkspaceDatasourcesManage) || perms.CanManage(datasourceID)
}
