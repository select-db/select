package commands

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"selectDb/internal/cmd/scripts"
)

// Command struct represents the backend logic to execute scripts
type Command struct{}

// RunCommand executes the appropriate script based on the provided command type.
func (c *Command) Run(cmdType string, arg string) (string, error) {
	validCommands := map[string]bool{
		"generate":      true,
		"migrate:up":    true,
		"migrate:down":  true,
		"migrate:reset": true,
		"migrate:new":   true,
	}

	if !validCommands[os.Args[1]] {
		log.Fatalf("Invalid command: %v", os.Args[1])
	}

	// Handle goose commands directly in Go
	if strings.HasPrefix(cmdType, "migrate:") {
		migrationCommand := strings.TrimPrefix(cmdType, "migrate:")

		var err error
		if migrationCommand == "new" {
			err = scripts.RunGoose("new", arg)
		} else {
			err = scripts.RunGoose(migrationCommand)
		}

		if err != nil {
			return "", fmt.Errorf("goose error: %v", err)
		}

		return fmt.Sprintf("goose migration '%s' executed successfully", migrationCommand), nil
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
