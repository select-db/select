package datasource

import (
	"context"
	"errors"
	"log"
	"net/http"

	"backend/internal/middlewares"
	"time"

	"github.com/selectDb/dialect/engine"
)

// genericConnErr is returned to clients for any datasource connection
// failure. The detail is logged server-side only: a raw dial error leaks
// internal network topology and turns this endpoint into an SSRF oracle.
const genericConnErr = "could not connect to the datasource"

// safeConnErr returns the error message if it is a ConfigError (safe to
// show), otherwise returns genericConnErr and logs the real error.
func safeConnErr(err error, logPrefix, workspaceID, dsID string) string {
	var cfgErr *engine.ConfigError
	if errors.As(err, &cfgErr) {
		return cfgErr.Msg
	}
	log.Printf("%s: open conn ws=%s id=%s: %v", logPrefix, workspaceID, dsID, err)
	return genericConnErr
}

func PingHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}
		noCache := r.URL.Query().Get("no_cache") == "true"

		workspaceID := middlewares.MemberWorkspaceID(r)
		pingKey := cacheKey(workspaceID, id)
		if !noCache {
			if cached, ok := pingCache.Get(pingKey); ok {
				if errMsg, _ := cached.(string); errMsg != "" {
					http.Error(w, errMsg, http.StatusBadGateway)
					return
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}

		ds, err := GetOrLoadDatasource(r.Context(), id, workspaceID)
		if err != nil {
			http.Error(w, "datasource not found", http.StatusNotFound)
			return
		}

		dbConn, err := engine.GetOrOpenConn(workspaceID, ds.DBType, ds.DSN, ds.SSH, ds.Pool)
		if err != nil {
			msg := safeConnErr(err, "datasource ping", workspaceID, id)
			pingCache.Set(pingKey, msg)
			http.Error(w, msg, http.StatusBadGateway)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		if err := dbConn.PingContext(ctx); err != nil {
			log.Printf("datasource ping: ping ws=%s id=%s: %v", workspaceID, id, err)
			pingCache.Set(pingKey, genericConnErr)
			http.Error(w, genericConnErr, http.StatusBadGateway)
			return
		}

		pingCache.Set(pingKey, true)
		w.WriteHeader(http.StatusNoContent)
	}
}
