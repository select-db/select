package cellarclient

import (
	"context"
	"encoding/json"
	"net/http"

	"backend/internal/cellar"
)

// Inventory lists every database of the cellar, on its disk or only in its bucket.
func Inventory(ctx context.Context) ([]cellar.StoredDatabase, error) {
	grant, err := cellar.Grant{CellarID: CellarID}.Encode()
	if err != nil {
		return nil, err
	}
	resp, err := callCellar(ctx, http.MethodGet, "/datasources", grant, nil)
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
	grant, err := cellar.Grant{CellarID: CellarID}.Encode()
	if err != nil {
		return err
	}
	resp, err := callCellar(ctx, http.MethodDelete, "/datasources/"+id, grant, nil)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}
