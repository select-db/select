package commands

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"selectDb/internal/db"
	"selectDb/internal/server"
)

// Command struct represents the backend logic to execute scripts
// migrationsSource is where 'new' scaffolds a file, relative to the app module.
const migrationsSource = "internal/db/migrations"

type Command struct{}

// RunCommand executes the appropriate script based on the provided command type.
func (c *Command) Run(cmdType string, arg string) (string, error) {
	validCommands := map[string]bool{
		"generate":       true,
		"migrate:up":     true,
		"migrate:down":   true,
		"migrate:reset":  true,
		"migrate:status": true,
		"migrate:new":    true,
	}

	if !validCommands[cmdType] {
		return "", fmt.Errorf("invalid command: %s", cmdType)
	}

	// Migrations run against the server the app is currently pointed at, through
	// the same embedded migrations it applies at startup. internal/cmd/migrate
	// is the same thing without booting the app, and can target another server.
	if strings.HasPrefix(cmdType, "migrate:") {
		migrationCommand := strings.TrimPrefix(cmdType, "migrate:")

		if migrationCommand == "new" {
			if arg == "" {
				return "", fmt.Errorf("migrate:new: missing migration name")
			}
			if err := db.NewMigration(migrationsSource, arg); err != nil {
				return "", err
			}
			return "migration created", nil
		}

		domain, err := server.ReadCurrentDomain()
		if err != nil {
			return "", err
		}
		if domain == "" {
			return "", fmt.Errorf("no current server to migrate")
		}
		dbPath, err := server.ServerDBPath(domain)
		if err != nil {
			return "", err
		}
		// No schema dump: a shipped binary has no source tree to write into.
		if err := db.RunGooseAt(dbPath, db.GooseCommand(migrationCommand), ""); err != nil {
			return "", fmt.Errorf("migrate %s on %s: %w", migrationCommand, domain, err)
		}
		return fmt.Sprintf("migrate %s applied to %s", migrationCommand, domain), nil
	}

	// Get the operating system
	OS := runtime.GOOS

	// Conditionally set executable permissions based on the OS
	if OS != "windows" {
		// Set execute permission for run_scripts.sh on Unix-like systems (Linux/macOS)
		cmdChmod := exec.Command("chmod", "+x", "./backend/cmd/cli/run_scripts.sh")
		if err := cmdChmod.Run(); err != nil {
			return "", fmt.Errorf("error setting executable permissions: %v", err)
		}
	}

	// Execute the script with the provided command type
	cmd := exec.Command("bash", "./backend/cmd/cli/run_scripts.sh", cmdType, arg)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return "", fmt.Errorf("error executing script for '%s': %v\nScript output:\n%s", cmdType, err, string(output))
	}

	return string(output), nil
}
