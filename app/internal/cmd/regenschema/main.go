package main

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"

	sqlite "modernc.org/sqlite"

	"selectDb/internal/db"
)

func main() {
	sql.Register("sqlite3", &sqlite.Driver{})
	tmp, err := os.MkdirTemp("", "regenschema")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if err := db.RunGooseAt(filepath.Join(tmp, "regen.db"), db.GooseUp, os.Args[1]); err != nil {
		log.Fatal(err)
	}
}
