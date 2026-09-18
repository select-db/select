// Package rest is the hand-written runtime the generated REST API is built from.
// It is deliberately a small library of primitives — JSON response writers, the
// authz gate, request-body decoding, the shared middleware-chain alias, and the
// per-op rate constants. The actual endpoint handlers are not here: apigen emits
// the full handler body into each generated internal/api/gen/<table>/<op>.go
// file, inlined and self-contained, so a reader opens one file and sees the
// whole request flow. Those handlers call into this package for the primitives
// and into the query runtime (OData $filter + keyset pagination) and the
// syncer's Apply/ApplyDelete (so tenancy, the cross-workspace FK guard, LWW, and
// audit emission are shared with the sync path).
package rest

import (
	"encoding/json"
	"errors"
	"net/http"

	"backend/internal/api/query"
	"backend/internal/api/validate"
	"backend/internal/syncer/types"
)

// WriteJSON encodes v as the response body with the given status. The generated
// API speaks JSON on every path, including errors (unlike the app's internal
// text/plain http.Error handlers).
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError emits the {"error": message} envelope the OpenAPI spec documents.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, map[string]string{"error": message})
}

// WriteQueryError maps a read error: a client-caused $filter/sort/cursor problem
// is a 400 with its precise message; anything else is an opaque 500.
func WriteQueryError(w http.ResponseWriter, err error) {
	var ie *query.InputError
	if errors.As(err, &ie) {
		WriteError(w, http.StatusBadRequest, ie.Message)
		return
	}
	WriteError(w, http.StatusInternalServerError, "internal error")
}

// WriteValidationError emits a 422 with the field-level issues from a write-body
// validation failure: {"error": <summary>, "fields": [{"field","message"}, …]}.
// The generated create/update handlers call it before the write reaches Apply.
func WriteValidationError(w http.ResponseWriter, e *validate.Error) {
	WriteJSON(w, http.StatusUnprocessableEntity, map[string]any{
		"error":  "the request body is invalid",
		"fields": e.Issues,
	})
}

// WriteWriteError maps an Apply/ApplyDelete failure to a response. A typed
// FieldError — the relational check the syncer runs that the body validator
// can't (a foreign key that doesn't resolve to a row in the workspace) — becomes
// a precise field-level 422 in the same envelope as body validation. Anything
// else stays an opaque 422 so a raw DB message never reaches the client.
func WriteWriteError(w http.ResponseWriter, singular string, err error) {
	var fe *types.FieldError
	if errors.As(err, &fe) {
		WriteValidationError(w, &validate.Error{Issues: []validate.Issue{{Field: fe.Field, Message: fe.Message}}})
		return
	}
	WriteError(w, http.StatusUnprocessableEntity, "could not process the "+singular)
}
