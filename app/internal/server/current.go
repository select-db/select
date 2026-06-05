package server

import (
	"os"
	"path/filepath"
	"strings"

	"selectDb/internal/utils"
)

const currentServerFileName = ".current_server"

// currentServerFilePath returns the path to the file that stores the current server domain.
func currentServerFilePath() (string, error) {
	appRoot, err := utils.GetAppDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(appRoot, currentServerFileName), nil
}

// ReadCurrentDomain returns the current server domain from the file, or empty if unset.
func ReadCurrentDomain() (string, error) {
	path, err := currentServerFilePath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// WriteCurrentDomain writes the current server domain to the file.
func WriteCurrentDomain(domain string) error {
	path, err := currentServerFilePath()
	if err != nil {
		return err
	}
	domain = strings.TrimSpace(domain)
	return os.WriteFile(path, []byte(domain), 0o600)
}

// CurrentServerRoot returns the root path for the current server, or empty if none.
func CurrentServerRoot() (string, error) {
	domain, err := ReadCurrentDomain()
	if err != nil || domain == "" {
		return "", err
	}
	return ServerRootPath(domain)
}
