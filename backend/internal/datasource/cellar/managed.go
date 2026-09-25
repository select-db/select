package cellar

import (
	"errors"
	"net/http"

	"backend/internal/authz"
	server "backend/internal/cellar"

	"github.com/selectDb/dialect/engine"
)

// ErrOff answers any use of a managed database while CELLAR is unset.
var ErrOff = errors.New("managed databases are not enabled on this server")

// maxBytes is what a managed database's workspace plan lets it grow to. An
// unknown plan has no size cap, which the cellar refuses.
var maxBytes = map[string]int64{
	"solo":  250 << 20,
	"teams": 1 << 30,
}

var client *Client

// Use sends managed databases to c. Call once at startup.
func Use(c *Client) { client = c }

// Datasource is what the backend knows of one managed datasource.
type Datasource struct {
	ID, WorkspaceID, DBType string
	CellarID, Plan          string
	Members                 int
}

// Engine returns the engine client and instance that run ds on its cellar,
// carrying the caller's permissions on it: the cellar enforces them, the
// backend does not.
func Engine(r *http.Request, ds Datasource) (*engine.Client, engine.DBInstance, error) {
	if client == nil {
		return nil, engine.DBInstance{}, ErrOff
	}
	t, err := client.Transport(server.Grant{
		WorkspaceID: ds.WorkspaceID,
		CellarID:    ds.CellarID,
		MaxBytes:    maxBytes[ds.Plan],
		MaxInFlight: max(4, 2*ds.Members),
		Permissions: authz.EntriesOn(r, ds.ID),
	})
	if err != nil {
		return nil, engine.DBInstance{}, err
	}
	return &engine.Client{Transport: t}, engine.DBInstance{ID: ds.ID, DBType: ds.DBType, Proxified: true}, nil
}
