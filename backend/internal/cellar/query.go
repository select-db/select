package cellar

import (
	"encoding/json"
	"net/http"

	"github.com/selectDb/dialect/engine/arrowstream"
	"github.com/selectDb/dialect/engine/query"
)

// Query is the body of POST /datasources/{id}/query: one statement and its
// placeholder values. The answer is the Arrow stream arrowstream.Stream reads.
type Query struct {
	SQL  string `json:"sql"`
	Args []any  `json:"args,omitempty"`
}

// QueryHandler runs one statement from the backend's driver on the grant's
// database, under the isolation rules, and streams the rows back.
func QueryHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var q Query
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		grant := GetGrant(r)
		w.Header().Set("Content-Type", "application/vnd.apache.arrow.stream")
		stream := arrowstream.NewSink(w)
		defer stream.Close()
		if f, ok := w.(http.Flusher); ok {
			stream.SetDownstreamFlusher(f.Flush)
		}
		sink := classifiedSink{Sink: stream, ctx: r.Context(), grant: grant}
		lease, err := memoryBudget.Begin()
		if err != nil {
			sink.OnError(err)
			return
		}
		defer lease.Release()
		path, err := databases.use(r.Context(), grant.DatasourceID)
		if err != nil {
			sink.OnError(err)
			return
		}
		conn, err := Open(path, grant)
		if err != nil {
			sink.OnError(err)
			return
		}
		query.Stream(r.Context(), conn, query.Datasource{ID: grant.DatasourceID, DBType: dbType}, q.SQL, query.Options{Args: q.Args, MaxValueBytes: query.ServerMaxValueBytes, Reserve: lease.Grow}, sink)
	}
}
