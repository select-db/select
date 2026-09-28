package e2e

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"backend/internal/auth"
	"backend/internal/cellar"
	"backend/internal/datasource/cellarclient"
)

// ServeCellar turns managed databases on for the test: a cellar over a temp
// dir, which it returns, reached the way the backend reaches a remote one.
func ServeCellar(t *testing.T) string {
	t.Helper()
	publicKey, err := auth.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	mux := http.NewServeMux()
	cellar.Register(mux, dir, publicKey, "local")
	cellarServer := httptest.NewServer(mux)
	t.Cleanup(cellarServer.Close)
	cellarclient.URL, cellarclient.CellarID = cellarServer.URL, "local"
	t.Cleanup(func() { cellarclient.URL, cellarclient.CellarID = "", "" })
	return dir
}
