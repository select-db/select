package e2e

import (
	"io"
	"net/http"
	"testing"

	"github.com/selectDb/dialect/engine/arrowstream"
	"github.com/stretchr/testify/require"
)

// Execute runs sql on a datasource through the backend's REST route as the
// owner, and returns its rows or the failure the caller sees, with its code.
func Execute(t *testing.T, fixture Fixture, datasourceID, sql string) ([][]any, *arrowstream.Error) {
	t.Helper()
	rec := Do(t, fixture.H, http.MethodPost, "/datasources/"+datasourceID+"/execute", fixture.Actor.Token,
		map[string]any{"workspace_id": fixture.Actor.WorkspaceID, "sql": sql})
	require.Equalf(t, http.StatusOK, rec.Code, "execute: %s", rec.Body.String())
	stream, err := arrowstream.NewStream(io.NopCloser(rec.Body))
	require.NoError(t, err)
	defer func() { _ = stream.Close() }()
	failed := func(err error) *arrowstream.Error {
		var coded *arrowstream.Error
		require.ErrorAs(t, err, &coded)
		return coded
	}
	if _, err := stream.Columns(); err != nil {
		return nil, failed(err)
	}
	var rows [][]any
	for {
		row, ok, err := stream.Next()
		if err != nil {
			return nil, failed(err)
		}
		if !ok {
			break
		}
		rows = append(rows, row)
	}
	if _, _, _, err := stream.Summary(); err != nil {
		return nil, failed(err)
	}
	return rows, nil
}
