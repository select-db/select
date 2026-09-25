package cellar

import (
	"crypto/rsa"
	"encoding/json"
	"log"
	"net/http"

	"backend/internal/authz"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/engine"
	"github.com/selectDb/dialect/engine/transport"
)

// dbType is the engine dialect of every database a cellar holds.
const dbType = "sqlite"

// Handler serves the databases in files to the backend. The routes are the
// ones the backend serves the app, so one engine transport reaches both.
func Handler(files *Files, pub *rsa.PublicKey, cellarID string) http.Handler {
	admit := Admit(pub, cellarID)
	mux := http.NewServeMux()
	mux.Handle("POST /datasources/{id}/execute", admit(execute(files)))
	mux.Handle("GET /datasources/{id}/schema", admit(schema(files)))
	mux.Handle("POST /datasources/{id}/ping", admit(ping(files)))
	mux.Handle("GET /datasources/{id}/dump", admit(dump(files)))
	return mux
}

// openFile opens the request's database with the caller's permissions and
// its schema.
func openFile(r *http.Request, files *Files, noCache bool) (engine.Conn, Grant, error) {
	g := GrantFrom(r.Context())
	conn, err := files.Open(g, authz.Compile(g.Perms))
	if err != nil {
		return engine.Conn{}, g, err
	}
	conn.Meta, err = engine.GetOrFetchMetadata(r.Context(), g.WS, files.Path(g.DB), conn.DB, engine.GetDialect(dbType), "", noCache)
	return conn, g, err
}

// fail answers a request the cellar could not serve, keeping paths out of
// the answer.
func fail(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("cellar: %s %s: %v", r.Method, r.URL.Path, err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}

func execute(files *Files) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req transport.ExecuteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		conn, g, err := openFile(r, files, false)
		if err != nil {
			fail(w, r, err)
			return
		}
		sink := transport.WriteArrow(w)
		defer sink.Close()
		engine.StreamLocal(r.Context(), conn, engine.DBInstance{ID: g.DB, DBType: dbType}, req.SQL, req.Options(), sink)
	}
}

func schema(files *Files) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, _, err := openFile(r, files, r.URL.Query().Get("no_cache") == "true")
		if err != nil {
			fail(w, r, err)
			return
		}
		transport.WriteZstdJSON(w, conn.Meta)
	}
}

func ping(files *Files) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := files.Open(GrantFrom(r.Context()), core.CompiledPermissions{})
		if err == nil {
			err = conn.DB.PingContext(r.Context())
		}
		if err != nil {
			fail(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func dump(files *Files) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, g, err := openFile(r, files, false)
		if err != nil {
			fail(w, r, err)
			return
		}
		schemaSQL := engine.GetOrGenerateDump(engine.GetDialect(dbType), g.WS, files.Path(g.DB), conn.Meta, false)
		transport.WriteZstdJSON(w, map[string]string{"sql": schemaSQL})
	}
}
