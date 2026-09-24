package datasource

import (
	"context"
	"encoding/json"
	"net/http"

	"backend/internal/middlewares"
	"time"

	"backend/internal/authz"

	"github.com/selectDb/dialect/engine"
	"github.com/selectDb/dialect/engine/arrowstream"
)

type executeRequest struct {
	ID        string `json:"id"`
	SQL       string `json:"sql"`
	MaxBytes  int64  `json:"max_bytes,omitempty"`
	TimeoutMs int64  `json:"timeout_ms,omitempty"`
}

func ExecuteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req executeRequest
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

		ds, err := GetOrLoadDatasource(r.Context(), req.ID, workspaceID)
		if err != nil {
			http.Error(w, "datasource not found", http.StatusNotFound)
			return
		}

		if ds.CellarID != "" {
			client, inst, err := OnCellar(r, req.ID, workspaceID, ds)
			if err != nil {
				cellarError(w, err, "datasource execute", workspaceID, req.ID)
				return
			}
			stream(w, r, req, ds, client, engine.Conn{}, inst)
			return
		}

		dbConn, err := engine.GetOrOpenConn(workspaceID, ds.DBType, ds.DSN, ds.SSH, ds.Pool)
		if err != nil {
			http.Error(w, safeConnErr(err, "datasource execute", workspaceID, req.ID), http.StatusBadGateway)
			return
		}

		dialect := engine.GetDialect(ds.DBType)
		if dialect == nil {
			http.Error(w, "unsupported database type", http.StatusBadRequest)
			return
		}

		meta, _ := engine.GetOrFetchMetadata(r.Context(), workspaceID, ds.DSN, dbConn, dialect, "", false)

		conn := engine.Conn{
			DB:    dbConn,
			Meta:  meta,
			Perms: authz.CompiledFromRequest(r),
		}
		stream(w, r, req, ds, &engine.Client{}, conn, engine.DBInstance{ID: req.ID, DBType: ds.DBType})
	}
}

// stream runs req through client into an audited arrow response.
func stream(w http.ResponseWriter, r *http.Request, req executeRequest, ds *ResolvedDatasource, client *engine.Client, conn engine.Conn, inst engine.DBInstance) {
	ctx := r.Context()
	if req.TimeoutMs > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.TimeoutMs)*time.Millisecond+5*time.Second)
		defer cancel()
	}

	inner := arrowResponse(w)
	defer inner.Close()

	// Wrap the sink to capture the query's outcome for the audit log.
	sink := newLoggingSink(inner, newQueryAuditRecord(r, req, ds.DBType))

	client.StreamTo(ctx, conn, inst, middlewares.MemberWorkspaceID(r), req.SQL, engine.Options{
		MaxBytes: req.MaxBytes,
		Timeout:  time.Duration(req.TimeoutMs) * time.Millisecond,
	}, sink)
}

// arrowResponse starts a streamed result; the caller closes the sink.
func arrowResponse(w http.ResponseWriter) *arrowstream.Sink {
	w.Header().Set("Content-Type", "application/vnd.apache.arrow.stream")
	w.Header().Set("Content-Encoding", "zstd")
	w.WriteHeader(http.StatusOK)

	sink := arrowstream.NewSink(w)
	// Push compressed bytes through HTTP buffering after every batch so
	// the client sees rows arrive steadily instead of in one tail clump
	// when the handler returns.
	if flusher, ok := w.(http.Flusher); ok {
		sink.SetDownstreamFlusher(flusher.Flush)
	}
	return sink
}
