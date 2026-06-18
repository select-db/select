package server

import (
	"path/filepath"

	"selectDb/internal/utils"
)

// ServerRootPath returns the absolute path to the server folder for the given domain.
// Layout: appRoot/<folderName>/ where folderName is domain with ":" encoded for the filesystem.
func ServerRootPath(domain string) (string, error) {
	appRoot, err := utils.GetAppDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(appRoot, DomainToFolderName(domain)), nil
}

// ServerDBPath returns the path to the SQLite DB file for the given domain.
func ServerDBPath(domain string) (string, error) {
	root, err := ServerRootPath(domain)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "db.sqlite"), nil
}
