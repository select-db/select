package server

import (
	"os"
	"path/filepath"

	"selectDb/backend/utils"
)

const dbFilename = "db.sqlite"

// ListServerDomains returns the list of server domains (folder names that contain a db.sqlite).
func ListServerDomains() ([]string, error) {
	appRoot, err := utils.GetAppDataDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(appRoot)
	if err != nil {
		return nil, err
	}
	var domains []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "" || name[0] == '.' {
			continue
		}
		dbPath := filepath.Join(appRoot, name, dbFilename)
		if _, err := os.Stat(dbPath); err != nil {
			continue
		}
		domains = append(domains, FolderNameToDomain(name))
	}
	return domains, nil
}
