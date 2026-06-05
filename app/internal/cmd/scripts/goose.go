package scripts

import (
	"fmt"
	"log"
	"os"
	"selectDb/internal/db"

	"github.com/pressly/goose/v3"
)

func RunGoose(command string, migrationName ...string) error {
	// Restrict sensitive commands outside of dev
	env := os.Getenv("APP_ENV")
	if env != "dev" && (command == "down" || command == "reset" || command == "new") {
		return fmt.Errorf("command '%s' is only allowed in dev environment", command)
	}

	// Reset to OS filesystem (init.go sets the embedded FS at startup)
	goose.SetBaseFS(nil)

	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}

	db := db.GetDB()

	migrationsDir := "./backend/db/migrations"

	var err error
	switch command {
	case "up":
		err = goose.Up(db, migrationsDir)
	case "down":
		err = goose.Down(db, migrationsDir)
	case "reset":
		err = goose.Reset(db, migrationsDir)
	case "status":
		err = goose.Status(db, migrationsDir)
	case "new":
		if len(migrationName) == 0 || migrationName[0] == "" {
			return fmt.Errorf("missing migration name for 'new'")
		}
		err = goose.Create(db, migrationsDir, migrationName[0], "sql")
	default:
		return fmt.Errorf("invalid command: %s", command)
	}
	if err != nil {
		return err
	}

	// Dump schema after up, down or reset, only in dev environment
	if env == "dev" && (command == "up" || command == "down" || command == "reset") {
		const outputFile = "./backend/db/schema.sql"
		if err := DumpSchema(db, outputFile); err != nil {
			return fmt.Errorf("failed to dump schema after '%s': %w", command, err)
		}
		log.Printf("[Dump] Schema saved to '%s'\n", outputFile)
	}

	return nil
}
