package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"backend/internal/auth"
	"backend/internal/cellar"
	managed "backend/internal/datasource/cellar"
)

// startCellar stops the server on a CELLAR it cannot parse, and sends managed
// databases to the cellar it names. local serves one from this process on a
// loopback port, over the files in CELLAR_DIR.
func startCellar() {
	v, err := parseCellar(os.Getenv("CELLAR"))
	if err != nil {
		log.Fatalf("cellar: %v", err)
	}
	if v == "" {
		log.Printf("cellar: off")
		return
	}
	log.Printf("cellar: %s", v)
	if v == localCellar {
		v, err = serveLocalCellar()
		if err != nil {
			log.Fatalf("cellar: %v", err)
		}
	}
	managed.Use(managed.NewClient(v))
}

func serveLocalCellar() (string, error) {
	dir := os.Getenv("CELLAR_DIR")
	if dir == "" {
		dir = ".dev/cellar"
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	pub, err := auth.PublicKey()
	if err != nil {
		return "", err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	srv := &http.Server{
		Handler:           cellar.Handler(cellar.NewFiles(dir), pub, localCellar),
		ReadHeaderTimeout: 10 * time.Second,
	}
	// Managed databases answer unavailable if it stops; the rest keeps serving.
	go func() { log.Printf("cellar: stopped: %v", srv.Serve(ln)) }()
	return "http://" + ln.Addr().String(), nil
}

// localCellar is the CELLAR value, and the cellar id, of a cellar run in the
// backend's own process.
const localCellar = "local"

// parseCellar checks a CELLAR value and returns it normalized: "" (managed
// databases off), localCellar, or an http(s) URL. Anything else is an error, so a typo cannot
// pass for "off".
func parseCellar(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" || v == localCellar {
		return v, nil
	}
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return "", fmt.Errorf(`CELLAR: want empty, %q or an http(s) URL without credentials`, localCellar)
	}
	return strings.TrimRight(v, "/"), nil
}
