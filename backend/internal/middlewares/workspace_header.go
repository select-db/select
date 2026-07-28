package middlewares

import (
	"context"
	"net/http"
)

// HeaderWorkspaceID is the request header the REST API uses to select the target
// workspace (the RPC endpoints instead carry workspace_id in the body).
const HeaderWorkspaceID = "X-Workspace-Id"

// WorkspaceFromHeader is the REST counterpart to Membership: it resolves the
// target workspace without reading the body (so it works for GET), validates it
// against the caller's memberships, and stashes it under the same context key so
// ActorOf / MemberWorkspaceID behave identically downstream. The header may be
// omitted when the caller belongs to exactly one workspace (the typical
// single-workspace API-key case), which is then used.
func WorkspaceFromHeader() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ids, _ := GetWorkspaceIDs(r)

			wsID := r.Header.Get(HeaderWorkspaceID)
			if wsID == "" {
				if len(ids) != 1 {
					http.Error(w, HeaderWorkspaceID+" header is required", http.StatusBadRequest)
					return
				}
				wsID = ids[0]
			}

			if !sliceContains(ids, wsID) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			ctx := context.WithValue(r.Context(), ctxWorkspaceID, wsID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
