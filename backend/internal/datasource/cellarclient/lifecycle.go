package cellarclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"backend/internal/cellar"

	"github.com/selectDb/dialect/engine/arrowstream"
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
	resp, err := lifecycleRequest(ctx, http.MethodPut, "/datasources/"+id, grant, body)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	var stored cellar.StoredDatabase
	err = json.NewDecoder(resp.Body).Decode(&stored)
	return stored.SizeBytes, err
}

// Download is a consistent copy of the database dsn names; the caller closes it.
func Download(ctx context.Context, dsn string) (io.ReadCloser, error) {
	id, grant, err := parseDSN(dsn)
	if err != nil {
		return nil, err
	}
	resp, err := lifecycleRequest(ctx, http.MethodGet, "/datasources/"+id+"/download", grant, nil)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// lifecycleRequest is a request to a lifecycle route, which answers a failure
// with a coded JSON error rather than a stream.
func lifecycleRequest(ctx context.Context, method, path, grant string, body []byte) (*http.Response, error) {
	resp, err := request(ctx, method, path, grant, body)
	if err != nil || resp.StatusCode < 300 {
		return resp, err
	}
	defer func() { _ = resp.Body.Close() }()
	errorBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var coded arrowstream.Error
	if json.Unmarshal(errorBody, &coded) == nil && coded.Code != "" {
		return nil, &coded
	}
	return nil, cellar.InternalError(fmt.Sprintf("cellar: %s %s: %d %s", method, path, resp.StatusCode, strings.TrimSpace(string(errorBody))))
}
