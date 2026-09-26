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
func Register(mux *http.ServeMux, dir string, pub *rsa.PublicKey, cellarID string) {
	authenticated := Authenticated(pub, cellarID)
	timeout := middlewares.Timeout(statementTimeout)
	inFlight := middlewares.InFlight(func(r *http.Request) (string, int) {
		grant := GetGrant(r)
		return grant.WorkspaceID, grant.MaxInFlight
	})

	mux.Handle("POST /datasources/{id}/query", authenticated(timeout(inFlight(QueryHandler(dir)))))
}
