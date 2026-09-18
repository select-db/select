package middlewares

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"backend/internal/auth"
)

func reqWithWorkspaces(ids ...string) *http.Request {
	ws := make([]auth.WorkspaceClaim, len(ids))
	for i, id := range ids {
		ws[i] = auth.WorkspaceClaim{ID: id}
	}
	ctx := context.WithValue(context.Background(), workspacesKey, ws)
	return httptest.NewRequest("GET", "/roles", nil).WithContext(ctx)
}

func TestWorkspaceFromHeader(t *testing.T) {
	var seen string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = MemberWorkspaceID(r)
		w.WriteHeader(http.StatusOK)
	})
	mw := WorkspaceFromHeader()(next)

	run := func(r *http.Request) *httptest.ResponseRecorder {
		seen = ""
		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, r)
		return rec
	}

	t.Run("header selects a member workspace", func(t *testing.T) {
		r := reqWithWorkspaces("w1", "w2")
		r.Header.Set(HeaderWorkspaceID, "w2")
		if rec := run(r); rec.Code != http.StatusOK || seen != "w2" {
			t.Fatalf("code=%d seen=%q", rec.Code, seen)
		}
	})

	t.Run("absent header defaults to the sole workspace", func(t *testing.T) {
		if rec := run(reqWithWorkspaces("only")); rec.Code != http.StatusOK || seen != "only" {
			t.Fatalf("code=%d seen=%q", rec.Code, seen)
		}
	})

	t.Run("absent header with several workspaces is a 400", func(t *testing.T) {
		if rec := run(reqWithWorkspaces("w1", "w2")); rec.Code != http.StatusBadRequest {
			t.Fatalf("code=%d", rec.Code)
		}
	})

	t.Run("non-member workspace is forbidden", func(t *testing.T) {
		r := reqWithWorkspaces("w1")
		r.Header.Set(HeaderWorkspaceID, "other")
		if rec := run(r); rec.Code != http.StatusForbidden {
			t.Fatalf("code=%d", rec.Code)
		}
	})
}
