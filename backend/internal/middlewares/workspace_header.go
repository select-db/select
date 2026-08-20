package middlewares

import (
	"context"
	"net/http"
)

// WorkspaceFromHeader resolves the target workspace from the X-Workspace-Id
// header only (no body read). Superseded by Membership, which also accepts the
// header; retained for its unit test until callers are fully migrated.
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
