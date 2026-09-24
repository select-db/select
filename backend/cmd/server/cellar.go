package main

import (
	"log"
	"os"

	"backend/internal/cellar"
)

// checkCellarConfig stops the server on a CELLAR it cannot parse, and logs
// whether managed databases are on.
func checkCellarConfig() {
	v, err := cellar.Parse(os.Getenv("CELLAR"))
	if err != nil {
		log.Fatalf("cellar: %v", err)
	}
	if v == "" {
		v = "off"
	}
	log.Printf("cellar: %s", v)
}
