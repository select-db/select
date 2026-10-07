package cellar

import (
	"crypto/rsa"
	"net/http"
	"time"

	"backend/internal/middlewares"
)

// statementTimeout caps every statement, its wait for one of the workspace's
// slots included. It is never read from the request.
const statementTimeout = 60 * time.Second

// Register adds the cellar's routes to mux. The backend's driver sends every
// statement here, and checks permissions before it does.
func Register(mux *http.ServeMux, pub *rsa.PublicKey) {
	authenticated := Authenticated(pub)
	timeout := middlewares.Timeout(statementTimeout)
	inFlight := middlewares.InFlight(func(r *http.Request) (string, int) {
		grant := GetGrant(r)
		return grant.WorkspaceID, grant.MaxInFlight
	})

	mux.Handle("POST /datasources/{id}/query", authenticated(timeout(inFlight(QueryHandler()))))
	// A copy or a download takes as long as the file is big, not a statement's 60s.
	mux.Handle("PUT /datasources/{id}", authenticated(CreateHandler()))
	mux.Handle("GET /datasources/{id}/download", authenticated(DownloadHandler()))
	mux.Handle("DELETE /datasources/{id}", authenticated(DeleteHandler()))
	mux.Handle("GET /datasources", authenticated(InventoryHandler()))
}
