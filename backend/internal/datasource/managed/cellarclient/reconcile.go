package cellarclient

import (
	"context"
	"encoding/json"
	"net/http"

	"backend/internal/cellar"
)

// serviceGrant is the grant of a request about the cellar as a whole, not about
// one workspace's database.
func serviceGrant() (string, error) {
	return cellar.Grant{CellarID: CellarID}.Encode()
}

// StoredDatabase is one database of the cellar, as its inventory lists it.
type StoredDatabase = cellar.StoredDatabase

// Inventory lists every database of the cellar, on its disk or only in its bucket.
func Inventory(ctx context.Context) ([]StoredDatabase, error) {
	grant, err := serviceGrant()
	if err != nil {
		return nil, err
	}
	resp, err := callCellar(ctx, http.MethodGet, "/datasources", grant, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var stored []StoredDatabase
	err = json.NewDecoder(resp.Body).Decode(&stored)
	return stored, err
}

// Purge removes database id from the cellar's disk and from its bucket.
func Purge(ctx context.Context, id string) error {
	grant, err := serviceGrant()
	if err != nil {
		return err
	}
	resp, err := callCellar(ctx, http.MethodDelete, "/datasources/"+id, grant, nil)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}
