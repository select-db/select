package cellarclient

import (
	"context"
	"encoding/json"
	"net/http"

	"backend/internal/cellar"
)

// emptyGrant is the grant of a request that acts for no workspace: the inventory and
// the purge.
var emptyGrant = func() string {
	grant, _ := cellar.Grant{}.Encode()
	return grant
}()

// Inventory lists every database of the cellar, on its disk or only in its bucket.
func Inventory(ctx context.Context) ([]cellar.StoredDatabase, error) {
	resp, err := callCellar(ctx, http.MethodGet, "/datasources", emptyGrant, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var stored []cellar.StoredDatabase
	err = json.NewDecoder(resp.Body).Decode(&stored)
	return stored, err
}

// Purge removes database id from the cellar's disk and from its bucket.
func Purge(ctx context.Context, id string) error {
	resp, err := callCellar(ctx, http.MethodDelete, "/datasources/"+id, emptyGrant, nil)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}
