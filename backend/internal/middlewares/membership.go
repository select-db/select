package middlewares

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const ctxWorkspaceID = contextKey("workspace_id")

// HeaderWorkspaceID is the request header used to select the target workspace.
const HeaderWorkspaceID = "X-Workspace-Id"

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

// Membership resolves the target workspace, validates it against the caller's
// memberships, and stashes it under ctxWorkspaceID for ActorOf/MemberWorkspaceID.
// The workspace comes from the X-Workspace-Id header (so GET reads work), the
// request body, or — when the caller has exactly one — that sole workspace.
func Membership() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wsID, err := resolveWorkspaceID(r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			ids, _ := GetWorkspaceIDs(r)
			if wsID == "" {
				if len(ids) != 1 {
					http.Error(w, HeaderWorkspaceID+" header or workspace_id is required", http.StatusBadRequest)
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

// resolveWorkspaceID reads the target workspace from the X-Workspace-Id header,
// falling back to a workspace_id field in the JSON body. The body is restored so
// downstream handlers can read it again. Returns "" when neither is present.
func resolveWorkspaceID(r *http.Request) (string, error) {
	if id := r.Header.Get(HeaderWorkspaceID); id != "" {
		return id, nil
	}
	if r.Body == nil {
		return "", nil
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "", errors.New("failed to read body")
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	if len(body) == 0 {
		return "", nil
	}

	var payload struct {
		WorkspaceID string `json:"workspace_id"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", errors.New("invalid request body")
	}
	return payload.WorkspaceID, nil
}
