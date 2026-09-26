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

// QueryHandler runs one statement on the grant's datasource, under the
// isolation rules, and streams the rows back.
func QueryHandler(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var q Query
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		grant := GetGrant(r)
		w.Header().Set("Content-Type", "application/vnd.apache.arrow.stream")
		inner := arrowstream.NewSink(w)
		defer inner.Close()
		if f, ok := w.(http.Flusher); ok {
			inner.SetDownstreamFlusher(f.Flush)
		}
		sink := classifiedSink{Sink: inner, ctx: r.Context(), grant: grant}
		conn, err := Open(dir, grant)
		if err != nil {
			sink.OnError(err)
			return
		}
		query.Stream(r.Context(), conn, query.Datasource{ID: grant.DatasourceID, DBType: dbType}, q.SQL, query.Options{Args: q.Args}, sink)
	}
}
