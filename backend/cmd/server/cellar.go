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
	"backend/internal/datasource/cellarclient"
)

// localCellar is the CELLAR setting, and the cellar id, of a cellar run in the
// backend's own process.
const localCellar = "local"

// startCellar stops the server on a CELLAR it cannot parse, and sends managed
// databases to the cellar it names. local serves one from this process on a
// loopback port, over the files in CELLAR_DIR, copied to CELLAR_BUCKET.
func startCellar() {
	setting, err := parseCellar(os.Getenv("CELLAR"))
	if err != nil {
		log.Fatalf("cellar: %v", err)
	}
	switch setting {
	case "":
		log.Printf("cellar: off")
		return
	case localCellar:
		address, err := serveLocalCellar()
		if err != nil {
			log.Fatalf("cellar: %v", err)
		}
		cellarclient.CellarID, cellarclient.URL = localCellar, address
	default:
		// parseCellar has already refused a setting that is not a URL.
		cellarURL, _ := url.Parse(setting)
		cellarclient.CellarID, cellarclient.URL = strings.ToLower(cellarURL.Hostname()), setting
	}
	log.Printf("cellar: %s", setting)
}

// serveLocalCellar starts a cellar in this process and returns its address.
func serveLocalCellar() (string, error) {
	dir := os.Getenv("CELLAR_DIR")
	if dir == "" {
		dir = ".dev/cellar"
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	bucket := os.Getenv("CELLAR_BUCKET")
	if bucket == "" {
		bucket = dir + "-bucket"
	}
	if err := cellar.OpenDatabases(dir, bucket); err != nil {
		return "", err
	}
	publicKey, err := auth.PublicKey()
	if err != nil {
		return "", err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	mux := http.NewServeMux()
	cellar.Register(mux, publicKey, localCellar)
	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	// Managed databases answer unavailable if it stops; the rest keeps serving.
	go func() { log.Printf("cellar: stopped: %v", server.Serve(listener)) }()
	return "http://" + listener.Addr().String(), nil
}

// parseCellar checks a CELLAR setting and returns it normalized: "" (managed
// databases off), localCellar, or an http(s) URL. Anything else is an error,
// so a typo cannot pass for "off".
func parseCellar(setting string) (string, error) {
	setting = strings.TrimSpace(setting)
	if setting == "" || setting == localCellar {
		return setting, nil
	}
	cellarURL, err := url.Parse(setting)
	if err != nil || (cellarURL.Scheme != "http" && cellarURL.Scheme != "https") || cellarURL.Host == "" || cellarURL.User != nil {
		return "", fmt.Errorf(`CELLAR: want empty, %q or an http(s) URL without credentials`, localCellar)
	}
	return strings.TrimRight(setting, "/"), nil
}
