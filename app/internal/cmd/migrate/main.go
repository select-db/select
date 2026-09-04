// Command migrate applies the app's SQLite migrations from a shell.
//
// The app runs them itself whenever it opens a server's database, so this is
// for working on the schema: rolling one back, checking what is applied, or
// scaffolding the next one.
//
//	migrate up|down|reset|status [domain]
//	migrate new <name>
//
// Every server has its own database. With no domain the command targets the
// current one, falling back to APP_ENV's default -- the same database the app
// would open if you launched it now.
package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	sqlite "modernc.org/sqlite"

	"selectDb/internal/db"
	"selectDb/internal/server"
)

// Both relative to the app module, which is where this is run from. schemaOut
// is sqlc's input (internal/sqlc.yaml), refreshed after a command that changes
// the schema.
const (
	migrationsSource = "internal/db/migrations"
	schemaOut        = "internal/db/schema.sql"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		usage()
	}
	command := os.Args[1]
	arg := ""
	if len(os.Args) > 2 {
		arg = os.Args[2]
	}

	if command == "new" {
		if arg == "" {
			log.Fatal("migrate new: missing <name>")
		}
		if err := db.NewMigration(migrationsSource, arg); err != nil {
			log.Fatal(err)
		}
		return
	}

	// The same driver name the app registers at startup, which is what
	// db.RunGooseAt opens the file through.
	sql.Register("sqlite3", &sqlite.Driver{})

	domain, err := resolveDomain(arg)
	if err != nil {
		log.Fatal(err)
	}
	dbPath, err := server.ServerDBPath(domain)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := os.Stat(dbPath); err != nil {
		log.Fatalf("no database for %q at %s -- open that server in the app first", domain, dbPath)
	}

	log.Printf("%s on %s (%s)", command, domain, filepath.Base(dbPath))
	if err := db.RunGooseAt(dbPath, db.GooseCommand(command), schemaOut); err != nil {
		log.Fatal(err)
	}
}

// resolveDomain prefers an explicit argument, then the server the app is
// currently pointed at, then the default for APP_ENV.
func resolveDomain(arg string) (string, error) {
	if arg != "" {
		return arg, nil
	}
	current, err := server.ReadCurrentDomain()
	if err != nil {
		return "", err
	}
	if current != "" {
		return current, nil
	}
	if fallback := server.DefaultDomainForEnv(); fallback != "" {
		return fallback, nil
	}
	return "", fmt.Errorf("no current server and APP_ENV names no default: pass a domain")
}

func usage() {
	log.Fatal("usage: migrate up|down|reset|status [domain]\n       migrate new <name>")
}
