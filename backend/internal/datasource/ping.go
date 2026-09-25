package datasource

import (
	"context"
	"net/http"
	"time"

	"backend/internal/middlewares"

	"github.com/selectDb/dialect/engine"
)

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

		o, err := Open(r, id, workspaceID)
		if err == nil {
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()
			err = (&engine.Client{}).Ping(ctx, o.Conn, o.Inst, workspaceID, noCache)
		}
		if err != nil {
			status, msg := openFailure(err, "datasource ping", workspaceID, id)
			if status == http.StatusBadGateway {
				pingCache.Set(pingKey, msg)
			}
			http.Error(w, msg, status)
			return
		}

		pingCache.Set(pingKey, true)
		w.WriteHeader(http.StatusNoContent)
	}
}
