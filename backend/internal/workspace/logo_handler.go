package workspace

import (
	"encoding/json"
	"errors"
	"net/http"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/authz"

	core "github.com/selectDb/dialect/core"
)

type updateLogoRequest struct {
	Logo string `json:"logo"`
}

type logoResponse struct {
	Logo string `json:"logo"`
}

// UpdateLogoHandler stores a workspace logo. The image never reaches the
// database as the caller sent it: NormalizeLogo decodes it, checks its shape and
// re-encodes it with our own PNG encoder, and the CHECK constraint on the column
// backs that up at the storage layer.
//
// This is the only write path for workspace.logo. The sync commit path does not
// carry the column at all (see internal/syncer/workspace/apply.go), so every
// stored logo has been through the checks below.
func UpdateLogoHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceUUID, ok := authorizeLogoWrite(w, r)
		if !ok {
			return
		}

		// The body is already capped by LimitLogoBody, which the route wraps
		// around this handler.
		var req updateLogoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		logo, err := NormalizeLogo(req.Logo)
		if err != nil {
			if errors.Is(err, ErrInvalidLogo) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.Error(w, "failed to read logo", http.StatusInternalServerError)
			return
		}

		if err := db.Queries.UpdateWorkspaceLogo(r.Context(), generated.UpdateWorkspaceLogoParams{
			ID:   workspaceUUID,
			Logo: db_types.NewJSONNullString(logo),
		}); err != nil {
			http.Error(w, "failed to save logo", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(logoResponse{Logo: logo})
	}
}

// LimitLogoBody caps the request body at MaxLogoRequestBytes before anything
// downstream reads it. It wraps the route rather than living inside the handler
// because the membership middleware buffers the body whole when the request
// carries no X-Workspace-Id header, which is above the handler in the chain.
func LimitLogoBody(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, MaxLogoRequestBytes)
		h.ServeHTTP(w, r)
	})
}

// authorizeLogoWrite gates the logo endpoint on the same permission the sync
// path required for a workspace write: owner, or workspace/settings.write.
func authorizeLogoWrite(w http.ResponseWriter, r *http.Request) (db_types.JSONNullUUID, bool) {
	a := authz.ActorOf(r)

	// The actor's permissions were compiled for the workspace the membership
	// middleware resolved, so a path id naming a different workspace would be
	// checked against the wrong set. Requiring them to match keeps the id in the
	// route honest rather than decorative.
	if id := r.PathValue("id"); id != a.WorkspaceID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return db_types.JSONNullUUID{}, false
	}

	if !a.IsOwner() && !a.Can(core.ActionWorkspaceSettingsWrite) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return db_types.JSONNullUUID{}, false
	}

	workspaceUUID, err := db_types.NewJSONNullUUIDFromString(a.WorkspaceID)
	if err != nil {
		http.Error(w, "invalid workspace id", http.StatusInternalServerError)
		return db_types.JSONNullUUID{}, false
	}
	return workspaceUUID, true
}
