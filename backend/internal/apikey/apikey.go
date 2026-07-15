// Package apikey is the control plane for API keys: list/create/revoke/rotate.
// Server-authoritative, never the syncer. Callers must be a user principal
// with workspace ownership or workspace/api-keys.manage; an API-key principal
// is rejected so a key cannot mint or escalate more keys.
package apikey

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"backend/internal/audit"
	"backend/internal/authz"
	"backend/internal/middlewares"

	core "github.com/selectDb/dialect/core"
)

const (
	maxExpiry         = 365 * 24 * time.Hour // upper bound when an expiry is set; nil = never
	rotateGraceWindow = 24 * time.Hour       // old key stays valid this long after rotate
)

var (
	errBadExpiry    = errors.New("expires_at must be a future RFC3339 timestamp")
	errExpiryTooFar = errors.New("expires_at exceeds the maximum allowed lifetime")
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
		emitDenied(r, deniedSpec, workspaceID)
		http.Error(w, "api keys cannot manage api keys", http.StatusForbidden)
		return "", "", false
	}
	userID, ok = middlewares.MustGetUserID(w, r)
	if !ok {
		return "", "", false
	}
	if !authz.IsWorkspaceOwner(r, workspaceID) {
		if !authz.CompiledFromRequest(r).IsAllowed(core.ActionWorkspaceApiKeysManage) {
			emitDenied(r, deniedSpec, workspaceID)
			http.Error(w, "forbidden", http.StatusForbidden)
			return "", "", false
		}
	}
	return workspaceID, userID, true
}

// emitDenied records a denied key-management attempt. A zero spec (the list/read
// path) is skipped — denied reads aren't audited.
func emitDenied(r *http.Request, spec audit.Spec, workspaceID string) {
	if spec.Action == "" {
		return
	}
	audit.EmitAction(r.Context(), spec, audit.Record{WorkspaceID: workspaceID, Status: audit.StatusDenied})
}

// resolveExpiry maps an optional RFC3339 string to a stored value. nil/empty =
// never expires; a value must be in the future and within maxExpiry.
func resolveExpiry(raw *string) (*time.Time, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, *raw)
	if err != nil {
		return nil, errBadExpiry
	}
	now := time.Now()
	if !t.After(now) {
		return nil, errBadExpiry
	}
	if t.After(now.Add(maxExpiry)) {
		return nil, errExpiryTooFar
	}
	return &t, nil
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
