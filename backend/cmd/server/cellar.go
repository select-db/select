package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"backend/internal/auth"
	"backend/internal/cellar"
	"backend/internal/datasource/managed"
	"backend/internal/datasource/managed/cellarclient"
	"backend/internal/kms"

	"github.com/benbjohnson/litestream"
	"github.com/selectDb/dialect/engine/membudget"
	"github.com/selectDb/toolkit"
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
		id, err := cellarID(cellarURL.Hostname())
		if err != nil {
			log.Fatalf("cellar: %v", err)
		}
		cellarclient.CellarID, cellarclient.URL = id, setting
	}
	managed.StartReconciler(context.Background())
	log.Printf("cellar: %s (id %s)", setting, cellarclient.CellarID)
}

// cellarID is the cellar's name in grants and in each row's cellar_id: CELLAR_ID,
// else the address host. It must pass the database's check, so a bad one fails at start.
func cellarID(host string) (string, error) {
	id := strings.ToLower(strings.TrimSpace(os.Getenv("CELLAR_ID")))
	source := "CELLAR_ID"
	if id == "" {
		id, source = strings.ToLower(host), "the host of its address"
	}
	if !cellar.ValidID(id) {
		return "", fmt.Errorf("the cellar id %q (%s) is not valid: lower case letters, digits and hyphens only; set CELLAR_ID", id, source)
	}
	return id, nil
}

// serveLocalCellar starts a cellar in this process and returns its address.
func serveLocalCellar() (string, error) {
	handler, err := cellarHandler(localCellar)
	if err != nil {
		return "", err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	// Managed databases answer unavailable if it stops; the rest keeps serving.
	go func() { log.Printf("cellar: stopped: %v", server.Serve(listener)) }()
	toolkit.RegisterStats("cellar", cellar.Stats)
	return "http://" + listener.Addr().String(), nil
}

// serveCellar runs this process as a cellar only, on CELLAR_LISTEN, until it
// is told to stop. It never opens Postgres. Its id is CELLAR_ID, the same value
// the backend has, or else the listen host.
func serveCellar() {
	toolkit.RegisterStats("cellar", cellar.Stats)
	toolkit.StartPprofServer()
	address := os.Getenv("CELLAR_LISTEN")
	if address == "" {
		address = "127.0.0.1:8081"
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil || host == "" {
		log.Fatalf("cellar: CELLAR_LISTEN: want host:port, got %q", address)
	}
	id, err := cellarID(host)
	if err != nil {
		log.Fatalf("cellar: %v", err)
	}
	if limit := membudget.LimitHeap(); limit > 0 {
		log.Printf("cellar: Go memory limit %d MiB", limit>>20)
	}
	handler, err := cellarHandler(id)
	if err != nil {
		log.Fatalf("cellar: %v", err)
	}
	server := &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		log.Printf("cellar: listening on %s", address)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("cellar: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	// Statements stop first, then the last sync sends every write to the bucket.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
	if err := cellar.CloseDatabases(ctx); err != nil {
		log.Printf("cellar: close: %v", err)
	}
	log.Printf("cellar: stopped")
}

// cellarHandler opens the databases in CELLAR_DIR, copied to CELLAR_BUCKET,
// and returns the cellar's routes for cellarID. The keys of an s3:// bucket
// are the secrets CELLAR_S3_ACCESS_KEY_ID and CELLAR_S3_SECRET_ACCESS_KEY.
func cellarHandler(cellarID string) (http.Handler, error) {
	dir := os.Getenv("CELLAR_DIR")
	if dir == "" {
		dir = ".dev/cellar"
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	bucket := os.Getenv("CELLAR_BUCKET")
	if bucket == "" {
		bucket = dir + "-bucket"
	}
	if litestream.IsURL(bucket) {
		accessKeyID, err := kms.Secret("CELLAR_S3_ACCESS_KEY_ID")
		if err != nil {
			return nil, err
		}
		secretAccessKey, err := kms.Secret("CELLAR_S3_SECRET_ACCESS_KEY")
		if err != nil {
			return nil, err
		}
		cellar.SetBucketKeys(accessKeyID, secretAccessKey)
	}
	if err := cellar.OpenDatabases(dir, bucket); err != nil {
		return nil, err
	}
	publicKey, err := auth.PublicKey()
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	cellar.Register(mux, publicKey, cellarID)
	return mux, nil
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
