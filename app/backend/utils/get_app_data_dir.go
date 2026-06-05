package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	appName = "selectDb"
)

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
