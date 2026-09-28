package cellar

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	server "backend/internal/cellar"

	"github.com/selectDb/dialect/engine/arrowstream"
)

// Create makes the database dsn names on its cellar, empty or a copy of the
// database from as it was at, and returns its size.
func Create(ctx context.Context, dsn, from, at string) (int64, error) {
	id, grant, err := grantOf(dsn)
	if err != nil {
		return 0, err
	}
	body, err := json.Marshal(server.Create{From: from, At: at})
	if err != nil {
		return 0, err
	}
	resp, err := lifecycle(ctx, http.MethodPut, "/datasources/"+id, grant, body)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	var stored server.Stored
	err = json.NewDecoder(resp.Body).Decode(&stored)
	return stored.SizeBytes, err
}

// Download is a consistent copy of the database dsn names; the caller closes it.
func Download(ctx context.Context, dsn string) (io.ReadCloser, error) {
	id, grant, err := grantOf(dsn)
	if err != nil {
		return nil, err
	}
	resp, err := lifecycle(ctx, http.MethodGet, "/datasources/"+id+"/download", grant, nil)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// lifecycle is request for a route that answers a failure as a coded JSON error.
func lifecycle(ctx context.Context, method, path, grant string, body []byte) (*http.Response, error) {
	resp, err := request(ctx, method, path, grant, body)
	if err != nil || resp.StatusCode < 300 {
		return resp, err
	}
	defer func() { _ = resp.Body.Close() }()
	msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var coded arrowstream.Error
	if json.Unmarshal(msg, &coded) == nil && coded.Code != "" {
		return nil, &coded
	}
	return nil, server.InternalError(fmt.Sprintf("cellar: %s %s: %d %s", method, path, resp.StatusCode, strings.TrimSpace(string(msg))))
}
