package cellar

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/selectDb/dialect/engine"
	"github.com/selectDb/dialect/engine/arrowstream"
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
		conn, err := Open(dir, grant)
		if err != nil {
			// The cause names a path, so it is only logged.
			log.Printf("cellar: %s: %v", r.URL.Path, err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/vnd.apache.arrow.stream")
		sink := arrowstream.NewSink(w)
		defer sink.Close()
		if f, ok := w.(http.Flusher); ok {
			sink.SetDownstreamFlusher(f.Flush)
		}
		engine.StreamLocal(r.Context(), conn, engine.Datasource{ID: grant.DatasourceID, DBType: dbType}, q.SQL, engine.Options{Args: q.Args}, sink)
	}
}
