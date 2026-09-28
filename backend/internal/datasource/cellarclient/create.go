package cellarclient

import (
	"context"
	"encoding/json"
	"net/http"

	"backend/internal/cellar"
)

// Create makes the database dsn names on its cellar, empty or a copy of
// sourceID as it was at pointInTime, and returns its size in bytes.
func Create(ctx context.Context, dsn, sourceID, pointInTime string) (int64, error) {
	id, grant, err := parseDSN(dsn)
	if err != nil {
		return 0, err
	}
	body, err := json.Marshal(cellar.CreateRequest{SourceID: sourceID, PointInTime: pointInTime})
	if err != nil {
		return 0, err
	}
	resp, err := callCellar(ctx, http.MethodPut, "/datasources/"+id, grant, body)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	var stored cellar.StoredDatabase
	err = json.NewDecoder(resp.Body).Decode(&stored)
	return stored.SizeBytes, err
}
