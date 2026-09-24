package main

import (
	"log"
	"os"

	"backend/internal/cellar"
)

// cellarConfig reads CELLAR. A value that does not parse stops the server: a
// typo must not quietly turn managed databases off in production.
func cellarConfig() cellar.Config {
	cfg, err := cellar.Parse(os.Getenv("CELLAR"))
	if err != nil {
		log.Fatalf("cellar: %v", err)
	}
	if cfg.Mode == cellar.Remote {
		log.Printf("cellar: remote at %s", cfg.URL)
	} else {
		log.Printf("cellar: %s", cfg.Mode)
	}
	return cfg
}
