package graph

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RenameDatabaseParams renames the database at URI to Name. The name is what
// the user typed; what the directory ends up called is up to us.
type RenameDatabaseParams struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

// RenameDatabase renames a database's directory, which is to say the database.
//
// Unlike creation, a taken name is refused rather than numbered. Someone
// creating a database did not ask for a particular name; someone renaming one
// did, and quietly giving them "analytics-2" answers a question they did not
// ask.
func (g *Graph) RenameDatabase(params RenameDatabaseParams) (*DatabaseLocation, error) {
	wsGraph, err := g.GetWorkspaceGraph()
	if err != nil {
		return nil, fmt.Errorf("workspace graph not initialized: %w", err)
	}

	fsCtx, err := NewWorkspaceFS(wsGraph.ID)
	if err != nil {
		return nil, fmt.Errorf("init workspace fs: %w", err)
	}

	dbPath, ok := fsCtx.Path(params.URI)
	if !ok {
		return nil, fmt.Errorf("database %s is not in this workspace", params.URI)
	}
	if !CheckIsDBInstance(dbPath) {
		return nil, fmt.Errorf("no database at %s", params.URI)
	}

	cfg, err := ReadFSDBConfig(filepath.Join(dbPath, DBConfigFileName))
	if err != nil {
		return nil, fmt.Errorf("read database config: %w", err)
	}

	current := filepath.Base(dbPath)
	folderName := DatabaseFolderName(params.Name)

	// Nothing to do, and nothing to complain about: the name it already has is
	// not a name that is taken.
	if folderName == current {
		return &DatabaseLocation{ID: cfg.ID, URI: params.URI, Name: current}, nil
	}

	parentPath := filepath.Dir(dbPath)
	taken, err := lowercasedNames(parentPath)
	if err != nil {
		return nil, err
	}
	// A rename that only changes case is the same directory, not a collision —
	// and on macOS and Windows it is the one case where a name being taken is
	// no reason to stop.
	if taken[strings.ToLower(folderName)] && !strings.EqualFold(folderName, current) {
		return nil, fmt.Errorf("%q is already here", folderName)
	}

	newPath := filepath.Join(parentPath, folderName)
	if err := os.Rename(dbPath, newPath); err != nil {
		return nil, fmt.Errorf("rename database directory: %w", err)
	}

	rel, _ := fsCtx.Rel(newPath)
	return &DatabaseLocation{ID: cfg.ID, URI: fsCtx.URI(rel), Name: folderName}, nil
}
