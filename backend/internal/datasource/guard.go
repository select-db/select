package datasource

import (
	"net/http"

	"backend/internal/audit"
	"backend/internal/authz"
)

// manageGuard enforces datasource-management authority: the caller must be the
// workspace owner or hold datasources.manage on this datasource. On denial it
// emits deniedSpec with StatusDenied (targeting the datasource) and writes 403;
// pass a zero Spec (the read/get path) to skip denial auditing. Returns false
// when the request was rejected and the handler should return.
func manageGuard(w http.ResponseWriter, r *http.Request, workspaceID, datasourceID string, deniedSpec audit.Spec) bool {
	if authz.IsWorkspaceOwner(r, workspaceID) || authz.CompiledFromRequest(r).CanManage(datasourceID) {
		return true
	}
	if deniedSpec.Action != "" {
		audit.EmitAction(r.Context(), deniedSpec, audit.Record{
			WorkspaceID: workspaceID,
			TargetID:    datasourceID,
			Status:      audit.StatusDenied,
		})
	}
	http.Error(w, "forbidden", http.StatusForbidden)
	return false
}
