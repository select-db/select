package cellar

import (
	"crypto/rsa"
	"encoding/json"
	"log"
	"net/http"

	"github.com/selectDb/dialect/engine"
	"github.com/selectDb/dialect/engine/arrowstream"
)

// dbType is the engine dialect of every database a cellar holds.
const dbType = "sqlite"

// Query is the body of POST /datasources/{id}/query: one statement and its
// placeholder values. The answer is the Arrow stream arrowstream.Stream reads.
type Query struct {
	SQL  string `json:"sql"`
	Args []any  `json:"args,omitempty"`
}

// Handler serves the databases in files as a SQLite server: the backend's
// driver sends each statement here, and checks permissions itself.
func Handler(files *Files, pub *rsa.PublicKey, cellarID string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /datasources/{id}/query", Admit(pub, cellarID)(query(files)))
	return mux
}

func query(files *Files) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var q Query
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		grant := GrantFrom(r.Context())
		conn, err := files.Open(grant)
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
		engine.StreamLocal(r.Context(), conn, engine.DBInstance{ID: grant.DatasourceID, DBType: dbType}, q.SQL, engine.Options{Args: q.Args}, sink)
	}
}
