package datasource

import (
	"errors"
	"net/http"

	"backend/internal/auth"
	"backend/internal/authz"
	"backend/internal/cellar"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/engine"
)

// ErrCellarOff answers any use of a managed database while CELLAR is unset.
var ErrCellarOff = errors.New("managed databases are not enabled on this server")

// plans are what a managed database's workspace plan lets it grow to and
// restore from; totals and counts are checked on create.
var plans = map[string]struct {
	maxBytes int64
	pitrDays int
}{
	"solo":  {maxBytes: 250 << 20, pitrDays: 1},
	"teams": {maxBytes: 1 << 30, pitrDays: 7},
}

var cellarClient *cellar.Client

// UseCellar sends managed databases to c. Call once at startup.
func UseCellar(c *cellar.Client) { cellarClient = c }

// OnCellar returns the engine client and instance that run the managed
// database id on its cellar, carrying the caller's permissions on it: the
// cellar enforces them, the backend does not.
func OnCellar(r *http.Request, id, workspaceID string, ds *ResolvedDatasource) (*engine.Client, engine.DBInstance, error) {
	if cellarClient == nil {
		return nil, engine.DBInstance{}, ErrCellarOff
	}
	var entries []core.PermissionEntry
	for _, e := range authz.EntriesFromRequest(r) {
		if e.DbInstanceID == nil || *e.DbInstanceID == "*" || *e.DbInstanceID == id {
			entries = append(entries, e)
		}
	}
	plan := plans[ds.Plan]
	t, err := cellarClient.Transport(auth.CellarGrant{
		DB:       id,
		WS:       workspaceID,
		CellarID: ds.CellarID,
		MaxBytes: plan.maxBytes,
		PITRDays: plan.pitrDays,
	}, entries)
	if err != nil {
		return nil, engine.DBInstance{}, err
	}
	return &engine.Client{Transport: t}, engine.DBInstance{ID: id, DBType: ds.DBType, Proxified: true}, nil
}
