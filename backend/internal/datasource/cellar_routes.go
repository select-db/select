package datasource

import (
	"crypto/rsa"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"backend/internal/cellar"

	"github.com/selectDb/dialect/engine"
)

const managedDBType = "sqlite"

// CellarHandler serves the databases in files to the backend. The routes are
// the ones the backend serves the app, so one engine transport reaches both.
func CellarHandler(files *cellar.Files, pub *rsa.PublicKey, cellarID string) http.Handler {
	admit := cellar.Admit(pub, cellarID)
	mux := http.NewServeMux()
	mux.Handle("POST /datasources/{id}/execute", admit(cellarExecute(files)))
	mux.Handle("GET /datasources/{id}/schema", admit(cellarSchema(files)))
	mux.Handle("POST /datasources/{id}/ping", admit(cellarPing(files)))
	mux.Handle("GET /datasources/{id}/dump", admit(cellarDump(files)))
	return mux
}

// openFile opens the database the request's grant names, with the caller's
// permissions and its schema.
func openFile(r *http.Request, files *cellar.Files, noCache bool) (engine.Conn, error) {
	g := cellar.GrantFrom(r.Context())
	perms, err := cellar.PermsFrom(r)
	if err != nil {
		return engine.Conn{}, err
	}
	conn, err := files.Open(g, perms)
	if err != nil {
		return engine.Conn{}, err
	}
	conn.Meta, err = engine.GetOrFetchMetadata(r.Context(), g.WS, files.Path(g.DB), conn.DB, engine.GetDialect(managedDBType), "", noCache)
	return conn, err
}

// cellarFail answers a request the cellar could not serve, keeping paths out of
// the answer.
func cellarFail(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("cellar: %s %s: %v", r.Method, r.URL.Path, err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}

func cellarExecute(files *cellar.Files) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req executeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		conn, err := openFile(r, files, false)
		if err != nil {
			cellarFail(w, r, err)
			return
		}

		sink := arrowResponse(w)
		defer sink.Close()
		if err := cellar.CheckStatement(req.SQL); err != nil {
			sink.OnError(err)
			return
		}
		inst := engine.DBInstance{ID: r.PathValue("id"), DBType: managedDBType}
		engine.StreamLocal(r.Context(), conn, inst, req.SQL, engine.Options{
			MaxBytes: req.MaxBytes,
			Timeout:  time.Duration(req.TimeoutMs) * time.Millisecond,
		}, sink)
	}
}

func cellarSchema(files *cellar.Files) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := openFile(r, files, r.URL.Query().Get("no_cache") == "true")
		if err != nil {
			cellarFail(w, r, err)
			return
		}
		writeZstdJSON(w, conn.Meta)
	}
}

func cellarPing(files *cellar.Files) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := files.Open(cellar.GrantFrom(r.Context()), nil)
		if err != nil {
			cellarFail(w, r, err)
			return
		}
		if err := conn.DB.PingContext(r.Context()); err != nil {
			cellarFail(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func cellarDump(files *cellar.Files) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := openFile(r, files, false)
		if err != nil {
			cellarFail(w, r, err)
			return
		}
		g := cellar.GrantFrom(r.Context())
		schemaSQL := engine.GetOrGenerateDump(engine.GetDialect(managedDBType), g.WS, files.Path(g.DB), conn.Meta, false)
		writeZstdJSON(w, map[string]string{"sql": schemaSQL})
	}
}
