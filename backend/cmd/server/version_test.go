package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func callVersion(t *testing.T) map[string]string {
	t.Helper()
	rr := httptest.NewRecorder()
	versionHandler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/version", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body
}

func TestVersionHandlerReleaseBaseURL(t *testing.T) {
	orig := latestAppVersion
	latestAppVersion = "1.2.3"
	defer func() { latestAppVersion = orig }()

	t.Run("composes from RELEASE_BASE_URL, trims trailing slash, appends tag", func(t *testing.T) {
		t.Setenv("RELEASE_BASE_URL", "https://dl.example.com/releases/download/")
		if got := callVersion(t)["release_base_url"]; got != "https://dl.example.com/releases/download/v1.2.3" {
			t.Fatalf("release_base_url = %q", got)
		}
	})

	t.Run("empty when RELEASE_BASE_URL unset", func(t *testing.T) {
		t.Setenv("RELEASE_BASE_URL", "")
		if got := callVersion(t)["release_base_url"]; got != "" {
			t.Fatalf("expected empty release_base_url, got %q", got)
		}
	})
}
