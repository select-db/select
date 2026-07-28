// Package rest is the hand-written runtime behind the generated REST API. The
// apigen handler emitter produces only thin per-entity Entity descriptors and a
// RegisterRoutes call; all request handling lives here as generic, entity-
// agnostic logic, so the behavior is written and tested once. Reads go through
// the query runtime (OData $filter + keyset pagination); writes reuse the
// syncer's Apply/ApplyDelete so tenancy, the cross-workspace FK guard, LWW, and
// audit emission are shared with the sync path rather than reimplemented.
package rest

import (
	"encoding/json"
	"errors"
	"net/http"

	"backend/internal/api/query"
)

// writeJSON encodes v as the response body with the given status. The generated
// API speaks JSON on every path, including errors (unlike the app's internal
// text/plain http.Error handlers).
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError emits the {"error": message} envelope the OpenAPI spec documents.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// writeQueryError maps a read error: a client-caused $filter/sort/cursor problem
// is a 400 with its precise message; anything else is an opaque 500.
func writeQueryError(w http.ResponseWriter, err error) {
	var ie *query.InputError
	if errors.As(err, &ie) {
		writeError(w, http.StatusBadRequest, ie.Message)
		return
	}
	writeError(w, http.StatusInternalServerError, "internal error")
}
