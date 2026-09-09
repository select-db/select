package scripts

import (
	"backend/db"
	"fmt"
	"os"

	"github.com/pressly/goose/v3"
)

func RunGoose(command string, migrationName ...string) error {
	// Restrict sensitive commands outside of dev
	env := os.Getenv("APP_ENV")
	if env != "dev" && (command == "down" || command == "reset" || command == "new") {
		return fmt.Errorf("command '%s' is only allowed in dev environment", command)
	}

	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	database := db.GetDB()

	// Use the embedded FS so the binary works from any working directory.
	// "new" is dev-only and writes a real file, so it still needs the filesystem path.
	const embeddedDir = "migrations"
	const fsDir = "./db/migrations"

	var err error
	switch command {
	case "up":
		goose.SetBaseFS(db.MigrationsFS)
		err = goose.Up(database, embeddedDir)
	case "down":
		goose.SetBaseFS(db.MigrationsFS)
		err = goose.Down(database, embeddedDir)
	case "reset":
		goose.SetBaseFS(db.MigrationsFS)
		err = goose.Reset(database, embeddedDir)
	case "status":
		goose.SetBaseFS(db.MigrationsFS)
		err = goose.Status(database, embeddedDir)
	case "new":
		if len(migrationName) == 0 || migrationName[0] == "" {
			return fmt.Errorf("missing migration name for 'new'")
		}
		goose.SetBaseFS(nil)
		err = goose.Create(database, migrationName[0], "sql", fsDir)
	default:
		return fmt.Errorf("invalid command: %s", command)
	}
	if err != nil {
		return err
	}

	return nil
}
