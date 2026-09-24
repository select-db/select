package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"backend/internal/auth"
	"backend/internal/cellar"
	"backend/internal/datasource"
)

// startCellar stops the server on a CELLAR it cannot parse, and sends managed
// databases to the cellar it names. local serves one from this process on a
// loopback port, over the files in CELLAR_DIR.
func startCellar() {
	v, err := cellar.Parse(os.Getenv("CELLAR"))
	if err != nil {
		log.Fatalf("cellar: %v", err)
	}
	if v == "" {
		log.Printf("cellar: off")
		return
	}
	log.Printf("cellar: %s", v)
	if v == cellar.Local {
		v, err = serveLocalCellar()
		if err != nil {
			log.Fatalf("cellar: %v", err)
		}
	}
	datasource.UseCellar(cellar.NewClient(v))
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
		Handler:           datasource.CellarHandler(cellar.NewFiles(dir), pub, cellar.Local),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() { log.Fatalf("cellar: %v", srv.Serve(ln)) }()
	return "http://" + ln.Addr().String(), nil
}
