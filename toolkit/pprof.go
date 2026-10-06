package toolkit

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"time"
)

// StartPprofServer starts a pprof HTTP server on loopback.
//
// PPROF_ADDR turns it on in any environment ("127.0.0.1:6060"). Without it the
// server starts on defaultAddr only when APP_ENV=dev, and is off otherwise.
// The address must be a loopback one: the profiles describe the process and
// must never be reachable from the network, so any other address is refused
// and nothing is started.
//
// Suggested default addrs:
//   - backend server: "localhost:6060"
//   - desktop app:    "localhost:6061"
//   - cellar:         "localhost:6062"
//
// Usage:
//
//	go tool pprof http://<addr>/debug/pprof/heap
//	go tool pprof http://<addr>/debug/pprof/profile?seconds=30
func StartPprofServer(defaultAddr string) {
	addr := os.Getenv("PPROF_ADDR")
	if addr == "" && os.Getenv("APP_ENV") == "dev" {
		addr = defaultAddr
	}
	if addr == "" {
		return
	}
	if err := requireLoopback(addr); err != nil {
		log.Printf("pprof server not started: %v", err)
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

// requireLoopback accepts "localhost:port" and addresses of 127.0.0.0/8 and ::1.
// An empty host (":6060") listens on every interface, so it is refused too.
func requireLoopback(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("PPROF_ADDR %q: want host:port: %w", addr, err)
	}
	if host == "localhost" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("PPROF_ADDR %q is not a loopback address: profiles must not be reachable from the network", addr)
}
