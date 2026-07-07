package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	appName = "selectDb"

	// UserConfigDirName is the sub-folder of the per-user app data directory
	// that holds personal config files (.theme, .config keybindings/snippets).
	// It lives outside every server and workspace folder so it is never
	// committed to a workspace git repo and follows the user across all
	// workspaces.
	UserConfigDirName = "user-config"
)

// UserConfigDir returns the absolute path to the per-user config directory and
// ensures it exists. It is resolved from the same per-user app data directory
// used for server/workspace data, so personal config is shared across every
// server and workspace.
func UserConfigDir() (string, error) {
	appRoot, err := GetAppDataDir()
	if err != nil {
		return "", fmt.Errorf("resolve app data dir: %w", err)
	}
	dir := filepath.Join(appRoot, UserConfigDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("ensure user config dir: %w", err)
	}
	return dir, nil
}

// getAppDataDir returns a secure and OS-compatible app data directory path
func GetAppDataDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user config directory: %w", err)
	}

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "default"
	} else if strings.Contains(env, "..") || filepath.IsAbs(env) {
		return "", fmt.Errorf("invalid APP_ENV value: %s", env)
	}

	path := filepath.Join(configDir, appName, env)
	if err := os.MkdirAll(path, 0o700); err != nil { // #nosec G703 -- configDir from os.UserConfigDir; env validated above; appName constant
		return "", fmt.Errorf("failed to create app data directory: %w", err)
	}

	return path, nil
}
