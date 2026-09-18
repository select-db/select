// Command initdb creates a server database and brings it up to date, for
// scripts that need one to exist before they can work on it.
package main

import (
	"database/sql"
	"log"
	"os"

	sqlite "modernc.org/sqlite"

	"selectDb/internal/db"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) != 2 {
		log.Fatal("usage: initdb <db-path>")
	}
	sql.Register("sqlite3", &sqlite.Driver{})
	if err := db.RunMigrationsAt(os.Args[1]); err != nil {
		log.Fatal(err)
	}
}
