package toolkit

import (
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"time"
)

// StartPprofServer starts a pprof HTTP server on the given addr when APP_ENV=dev.
// It is a no-op in any other environment.
//
// Suggested addrs:
//   - backend server: "localhost:6060"
//   - desktop app:    "localhost:6061"
//
// Usage:
//
//	go tool pprof http://<addr>/debug/pprof/heap
//	go tool pprof http://<addr>/debug/pprof/profile?seconds=30
func StartPprofServer(addr string) {
	if os.Getenv("APP_ENV") != "dev" {
		return
	}

	// Register pprof on a dedicated mux rather than DefaultServeMux so the
	// profiling endpoints never leak onto a shared/default server.
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("pprof server listening on http://%s/debug/pprof/", addr)
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("pprof server error: %v", err)
		}
	}()
}
