package rest

import (
	"encoding/json"
	"net/http"

	"backend/internal/authz"
)

// Wrap is the shared middleware chain a generated route table applies to each
// handler — authenticated ∘ WorkspaceFromHeader ∘ limited(perMinute) — supplied
// by the api package so this package needn't import it. Parameterized by rate.
type Wrap = func(perMinute int, h http.Handler) http.Handler

// Per-op request-rate ceilings (requests/minute), mirroring the hand-written
// routes' pattern of cheap reads and stricter writes. Keyed by principal.
const (
	ReadRate  = 120
	WriteRate = 30
)

// Gate enforces an op's required workspace actions: the owner bypasses, anyone
// else must hold every listed action. Membership itself is already enforced by
// the middleware, so an op with no required actions (nil) is open to any member.
// Each generated handler passes its own action list inline, so the file states
// exactly what it requires.
func Gate(a authz.Actor, actions []string) bool {
	if a.IsOwner() {
		return true
	}
	for _, act := range actions {
		if !a.Can(act) {
			return false
		}
	}
	return true
}

// DecodeBody reads a JSON object request body. On a malformed body it writes the
// 400 and reports false; a null/empty body decodes to an empty (non-nil) map.
func DecodeBody(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	var m map[string]any
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return nil, false
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, true
}
