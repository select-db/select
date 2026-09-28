package cellarclient

import (
	"context"
	"io"
	"net/http"
)

// Download is a consistent copy of the database dsn names; the caller closes it.
func Download(ctx context.Context, dsn string) (io.ReadCloser, error) {
	id, grant, err := parseDSN(dsn)
	if err != nil {
		return nil, err
	}
	resp, err := callCellar(ctx, http.MethodGet, "/datasources/"+id+"/download", grant, nil)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}
