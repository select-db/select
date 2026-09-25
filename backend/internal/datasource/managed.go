package datasource

import (
	"errors"
	"net/http"

	"backend/internal/authz"
	"backend/internal/cellar"

	"github.com/selectDb/dialect/engine"
)

// ErrCellarOff answers any use of a managed database while CELLAR is unset.
var ErrCellarOff = errors.New("managed databases are not enabled on this server")

// maxBytes is what a managed database's workspace plan lets it grow to. An
// unknown plan has no size cap, which the cellar refuses.
var maxBytes = map[string]int64{
	"solo":  250 << 20,
	"teams": 1 << 30,
}

var cellarClient *CellarClient

// UseCellar sends managed databases to c. Call once at startup.
func UseCellar(c *CellarClient) { cellarClient = c }

// onCellar returns the engine client and instance that run the managed
// database id on its cellar, carrying the caller's permissions on it: the
// cellar enforces them, the backend does not.
func onCellar(r *http.Request, id, workspaceID string, ds *ResolvedDatasource) (*engine.Client, engine.DBInstance, error) {
	if cellarClient == nil {
		return nil, engine.DBInstance{}, ErrCellarOff
	}
	t, err := cellarClient.Transport(cellar.Grant{
		WS:       workspaceID,
		CellarID: ds.CellarID,
		MaxBytes: maxBytes[ds.Plan],
		Slots:    max(4, 2*ds.Members),
		Perms:    authz.EntriesOn(r, id),
	})
	if err != nil {
		return nil, engine.DBInstance{}, err
	}
	return &engine.Client{Transport: t}, engine.DBInstance{ID: id, DBType: ds.DBType, Proxified: true}, nil
}
