package datasource

import (
	"context"
	"encoding/json"
	"net/http"

	"backend/internal/middlewares"
	"time"

	"github.com/selectDb/dialect/engine/transport"
)

func ExecuteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req transport.ExecuteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		req.ID = r.PathValue("id")
		if req.ID == "" || req.SQL == "" {
			http.Error(w, "id and sql are required", http.StatusBadRequest)
			return
		}

		workspaceID := middlewares.MemberWorkspaceID(r)

		o, err := Open(r, req.ID, workspaceID)
		if err != nil {
			OpenError(w, err, "datasource execute", workspaceID, req.ID)
			return
		}

		ctx := r.Context()
		if req.TimeoutMs > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, time.Duration(req.TimeoutMs)*time.Millisecond+5*time.Second)
			defer cancel()
		}

		inner := transport.WriteArrow(w)
		defer inner.Close()

		// Wrap the sink to capture the query's outcome for the audit log.
		sink := newLoggingSink(inner, newQueryAuditRecord(r, req, o.DS.DBType))
		o.Stream(ctx, req.SQL, req.Options(), sink)
	}
}
