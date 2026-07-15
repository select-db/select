package apikey

import (
	"net/http"

	"backend/internal/audit"
	"backend/internal/authz"
	"backend/internal/middlewares"

	core "github.com/selectDb/dialect/core"
)

// guard enforces method, principal type, and the manage permission. It returns
// the validated workspace ID (from Membership) and the acting user ID. On a
// permission denial it emits deniedSpec with StatusDenied; pass a zero Spec (the
// list/read path) to skip denial auditing.
func guard(w http.ResponseWriter, r *http.Request, deniedSpec audit.Spec) (workspaceID, userID string, ok bool) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return "", "", false
	}
	workspaceID = middlewares.MemberWorkspaceID(r)
	// An API-key principal attempting key management is a privilege-escalation
	// signal, worth auditing as a denial.
	if middlewares.IsAPIKeyPrincipal(r) {
		audit.EmitDenied(r.Context(), deniedSpec, workspaceID, "")
		http.Error(w, "api keys cannot manage api keys", http.StatusForbidden)
		return "", "", false
	}
	userID, ok = middlewares.MustGetUserID(w, r)
	if !ok {
		return "", "", false
	}
	if !authz.IsWorkspaceOwner(r, workspaceID) {
		if !authz.CompiledFromRequest(r).IsAllowed(core.ActionWorkspaceApiKeysManage) {
			audit.EmitDenied(r.Context(), deniedSpec, workspaceID, "")
			http.Error(w, "forbidden", http.StatusForbidden)
			return "", "", false
		}
	}
	return workspaceID, userID, true
}
