package middlewares

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
)

const ctxWorkspaceID = contextKey("workspace_id")

func sliceContains(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

// MemberWorkspaceID returns the validated workspace ID from the request context.
func MemberWorkspaceID(r *http.Request) string {
	return r.Context().Value(ctxWorkspaceID).(string)
}

// Membership extracts workspace_id from JSON body, checks membership,
// injects into context. Body is re-readable for downstream handlers.
func Membership() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "failed to read body", http.StatusBadRequest)
				return
			}

			var payload struct {
				WorkspaceID string `json:"workspace_id"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				http.Error(w, "invalid request body", http.StatusBadRequest)
				return
			}
			if payload.WorkspaceID == "" {
				http.Error(w, "workspace_id is required", http.StatusBadRequest)
				return
			}

			ids, _ := GetWorkspaceIDs(r)
			if !sliceContains(ids, payload.WorkspaceID) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			r.Body = io.NopCloser(bytes.NewReader(body))
			ctx := context.WithValue(r.Context(), ctxWorkspaceID, payload.WorkspaceID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
