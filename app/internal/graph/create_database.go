package graph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// defaultNewDatabaseType is what a database is until someone opens its form and
// says otherwise.
const defaultNewDatabaseType = "postgresql"

// CreateDatabaseParams asks for a database in the folder at FolderURI, named
// Name. The name is what the user typed; where it lands is up to us.
type CreateDatabaseParams struct {
	FolderURI string `json:"folder_uri"`
	Name      string `json:"name"`
}

// CreatedDatabase is where the database went. Name is the directory's name,
// which is to say the database's, and may differ from what was asked for: the
// filesystem refuses some names, and a sibling may already have this one.
type CreatedDatabase struct {
	ID   string `json:"id"`
	URI  string `json:"uri"`
	Name string `json:"name"`
}

// CreateDatabase makes the directory for a new database and writes its config.
//
// The directory and the config are written together here rather than from the
// frontend, because choosing the name needs to see the directory it is going
// into: two databases cannot share a name, and which names are free is only
// knowable on disk.
func (g *Graph) CreateDatabase(params CreateDatabaseParams) (*CreatedDatabase, error) {
	wsGraph, err := g.GetWorkspaceGraph()
	if err != nil {
		return nil, fmt.Errorf("workspace graph not initialized: %w", err)
	}

	fsCtx, err := NewWorkspaceFS(wsGraph.ID)
	if err != nil {
		return nil, fmt.Errorf("init workspace fs: %w", err)
	}

	parentPath, ok := fsCtx.Path(params.FolderURI)
	if !ok {
		return nil, fmt.Errorf("folder %s is not in this workspace", params.FolderURI)
	}

	folderName, err := AvailableFolderName(parentPath, DatabaseFolderName(params.Name))
	if err != nil {
		return nil, err
	}

	dbPath := filepath.Join(parentPath, folderName)
	if err := os.MkdirAll(dbPath, 0o700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	id := uuid.NewString()
	config := FSDBConfig{
		ID:     id,
		Name:   folderName,
		DbType: defaultNewDatabaseType,
	}

	// The watcher turns this write into the db_instance node; the directory on
	// its own is only a folder.
	body, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode database config: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dbPath, DBConfigFileName), body, 0o600); err != nil {
		return nil, fmt.Errorf("write database config: %w", err)
	}

	rel, _ := fsCtx.Rel(dbPath)
	return &CreatedDatabase{ID: id, URI: fsCtx.URI(rel), Name: folderName}, nil
}

// AvailableFolderName is name, or the first of name-2, name-3 ... that nothing
// in dir is using. The comparison ignores case, because macOS and Windows do:
// a workspace holding both "Sales" and "sales" is one that cannot be checked
// out on either.
func AvailableFolderName(dir, name string) (string, error) {
	taken, err := lowercasedNames(dir)
	if err != nil {
		return "", err
	}

	if !taken[strings.ToLower(name)] {
		return name, nil
	}
	for n := 2; ; n++ {
		candidate := name + "-" + strconv.Itoa(n)
		if !taken[strings.ToLower(candidate)] {
			return candidate, nil
		}
	}
}

// lowercasedNames is what dir holds, lowercased for comparison. A directory
// that does not exist yet holds nothing, which is not an error: the folder a
// database is being made in may itself be on its way.
func lowercasedNames(dir string) (map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, fmt.Errorf("read folder %s: %w", dir, err)
	}

	names := make(map[string]bool, len(entries))
	for _, entry := range entries {
		names[strings.ToLower(entry.Name())] = true
	}
	return names, nil
}
