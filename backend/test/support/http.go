package support

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/selectDb/dialect/engine"
	"github.com/stretchr/testify/require"

	"backend/internal/api"
	"backend/internal/syncer/types"
)

func init() {
	// The e2e process dials loopback (embedded Postgres as a target datasource);
	// the production SSRF guard would reject private addresses.
	engine.EnforceOutboundGuard = false
}

// NewHandler builds the real application handler graph (same routes + middleware
// chain as the server binary) for in-process requests.
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	api.Register(mux)
	return api.Wrap(mux)
}

// Do sends an in-process request with a Bearer token and JSON body and returns
// the recorded response. body may be nil.
func Do(t *testing.T, h http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// SyncCommit posts a single-commit sync request as the actor and asserts the
// endpoint accepted it (HTTP 200). Payload is the entity body; workspaceID is
// carried on the commit (the sync path resolves the principal per commit).
func SyncCommit(t *testing.T, h http.Handler, actor Actor, operation, table, objectID string, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	req := types.SyncRequest{
		PendingCommits: []types.Commit{{
			ID:          objectID + ":" + operation,
			CreatedAt:   time.Now(),
			Operation:   operation,
			TableName:   table,
			ObjectID:    objectID,
			UserID:      actor.UserID,
			WorkspaceID: actor.WorkspaceID,
			Payload:     payload,
		}},
	}
	rec := Do(t, h, http.MethodPost, "/sync/v1/sync", actor.Token, req)
	require.Equalf(t, http.StatusOK, rec.Code, "sync %s %s failed: %s", operation, table, rec.Body.String())
	return rec
}
