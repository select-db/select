package transport

import (
	"context"
	"time"

	"github.com/selectDb/dialect/engine"
	"github.com/selectDb/dialect/engine/arrowstream"
)

// ExecuteRequest is the body of POST datasources/{id}/execute.
type ExecuteRequest struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	SQL         string `json:"sql"`
	MaxBytes    int64  `json:"max_bytes,omitempty"`
	TimeoutMs   int64  `json:"timeout_ms,omitempty"`
}

// Options are the engine options the request carries.
func (r ExecuteRequest) Options() engine.Options {
	return engine.Options{
		MaxBytes: r.MaxBytes,
		Timeout:  time.Duration(r.TimeoutMs) * time.Millisecond,
	}
}

func (t *HTTPTransport) OpenStream(
	ctx context.Context,
	workspaceID,
	instanceID,
	dbType,
	sql string,
	opts engine.Options,
) (engine.RowStream, error) {
	body, err := t.FetchStream(ctx, "POST", "datasources/"+instanceID+"/execute", ExecuteRequest{
		ID:          instanceID,
		WorkspaceID: workspaceID,
		SQL:         sql,
		MaxBytes:    opts.MaxBytes,
		TimeoutMs:   int64(opts.Timeout.Milliseconds()),
	}, workspaceHeader(workspaceID))
	if err != nil {
		return nil, err
	}
	return arrowstream.NewStream(body)
}
