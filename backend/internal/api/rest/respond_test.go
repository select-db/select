package rest

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"backend/internal/api/query"
)

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, 404, "role not found")
	if rec.Code != 404 {
		t.Fatalf("status: got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type: got %q", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "role not found" {
		t.Fatalf("body: got %v", body)
	}
}

func TestWriteQueryError(t *testing.T) {
	// A client $filter error is a 400 with the precise message.
	rec := httptest.NewRecorder()
	writeQueryError(rec, &query.InputError{Message: `unknown field "stat"`})
	if rec.Code != 400 {
		t.Fatalf("filter error status: got %d want 400", rec.Code)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] != `unknown field "stat"` {
		t.Fatalf("filter error body: got %v", body)
	}

	// Any other error is an opaque 500.
	rec = httptest.NewRecorder()
	writeQueryError(rec, fmt.Errorf("connection refused"))
	if rec.Code != 500 {
		t.Fatalf("internal error status: got %d want 500", rec.Code)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] == "connection refused" {
		t.Fatal("internal error leaked to client")
	}
}
