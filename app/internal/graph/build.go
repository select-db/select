package graph

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func (g *Graph) RebuildWorkspaceGraph() error {
	g.mu.Lock()
	if err := g.BuildWorkspaceGraph(); err != nil {
		g.mu.Unlock()
		return err
	}
	wg := g.WorkspaceGraph
	onGraphBuilt := g.AfterWorkspaceGraphBuild
	g.mu.Unlock()
	if onGraphBuilt != nil {
		onGraphBuilt(wg)
	}
	return nil
}

func (g *Graph) BuildWorkspaceGraph() error {
	ctx := context.Background()

	user, err := g.Queries.GetCurrentUser(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("no current user")
		}
		return fmt.Errorf("get current user: %w", err)
	}

	ws, err := g.Queries.GetCurrentWorkspace(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("get current workspace: %w", err)
	}

	g.WorkspaceGraph = &WorkspaceNode{
		ID:   ws.ID,
		Type: "workspace",

		Name:    ws.Name,
		IsOwner: ws.OwnerID.String == user.ID,

		User:        BuildUserNode(user),
		Folders:     []*FolderNode{},
		DBInstances: []*DBInstanceNode{},
	}

	fsCtx, err := NewWorkspaceFS(ws.ID)
	if err != nil {
		return fmt.Errorf("init workspace fs: %w", err)
	}

	// Build the workspace tree by recursively reading the filesystem rooted at
	// the workspace folder, plus sidecar/config files.
	if err := g.buildWorkspaceGraphFromFS(fsCtx); err != nil {
		return err
	}

	return nil
}

// BuildWorkspaceGraphFromFS is the exported entry point for tests that need to
// populate a Graph from a known workspace root without the full server setup.
func (g *Graph) BuildWorkspaceGraphFromFS(fsCtx *WorkspaceFS) error {
	return g.buildWorkspaceGraphFromFS(fsCtx)
}

// buildWorkspaceGraphFromFS populates the in-memory workspace graph purely
// from the filesystem and associated config/metadata files. It discovers
// folders, files and db instances under the workspace root and builds the
// corresponding FolderNode/FileNode/DBInstanceNode tree.
func (g *Graph) buildWorkspaceGraphFromFS(fsCtx *WorkspaceFS) error {
	// Reset DB instances for this workspace graph.
	g.WorkspaceGraph.DBInstances = []*DBInstanceNode{}

	workspaceRoot := fsCtx.WorkspaceRoot
	rootID := fsCtx.RootURI

	// Synthetic root folder representing the workspace root directory.
	rootFolder := &FolderNode{
		ID:   rootID,
		URI:  rootID,
		Type: "folder",

		Name: "root",

		FolderID:    "",
		Files:       []*FileNode{},
		Folders:     []*FolderNode{},
		DBInstances: []*DBInstanceNode{},
		Variables:   make(map[string]string),
	}

	foldersByID := map[string]*FolderNode{
		rootID: rootFolder,
	}
	dbInstancesByURI := map[string]*DBInstanceNode{}

	// If the workspace root does not exist yet, we still expose an empty root.
	if _, err := os.Stat(workspaceRoot); err != nil {
		if os.IsNotExist(err) {
			g.WorkspaceGraph.Folders = []*FolderNode{rootFolder}
			return nil
		}
		return fmt.Errorf("stat workspace root: %w", err)
	}

	normalizeDirURI := func(uri string) string {
		return strings.TrimSuffix(uri, "/")
	}

	// Load .env file for the root folder if it exists
	_ = g.LoadFolderEnvFile(rootFolder, fsCtx)

	err := filepath.WalkDir(workspaceRoot, func(cpath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Skip entries that cannot be accessed.
			return nil
		}

		// Skip the workspace root itself; it's represented by rootFolder.
		if cpath == workspaceRoot {
			return nil
		}

		relSlash, ok := fsCtx.Rel(cpath)
		if !ok {
			return nil
		}

		relSlash = filepath.ToSlash(filepath.Clean(relSlash))

		// Ignore internal workspace paths such as ".git".
		if IsInternalWorkspacePath(relSlash) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			// Determine parent folder URI for this directory.
			parentURI := fsCtx.ParentURI(relSlash) // parent of this directory
			parentFolder := g.getOrCreateParentFolder(foldersByID, parentURI, fsCtx)

			// Check if this directory represents a db instance (contains a
			// db.config.json). In that case, we build a DBInstanceNode and
			// continue walking to populate its files/folders.
			if CheckIsDBInstance(cpath) {
				cfg, readErr := ReadFSDBConfig(filepath.Join(cpath, "db.config.json"))
				if readErr != nil {
					return fs.SkipDir
				}

				dbURI := fsCtx.URI(relSlash)

				sshConfig := SSHConfigFromFS(cfg.SSH)

				node := &DBInstanceNode{
					ID:   cfg.ID,
					URI:  dbURI,
					Type: "db_instance",

					Name:      cfg.Name,
					DBType:    cfg.DbType,
					DSN:       cfg.DSN,
					Proxified: cfg.Proxified,

					SSH: sshConfig,

					WorkspaceID: g.WorkspaceGraph.ID,
					FolderID:    parentURI,

					Children: []*DBInstanceItemNode{},
					Files:    []*FileNode{},
					Folders:  []*FolderNode{},
				}

				dbInstancesByURI[normalizeDirURI(dbURI)] = node
				parentFolder.DBInstances = append(parentFolder.DBInstances, node)
				g.WorkspaceGraph.DBInstances = append(g.WorkspaceGraph.DBInstances, node)

				// Continue walking into the database folder
				return nil
			}

			// Regular folder.
			folderURI := fsCtx.URI(relSlash)
			if _, exists := foldersByID[folderURI]; exists {
				return nil
			}

			folder := &FolderNode{
				ID:   folderURI,
				URI:  folderURI,
				Type: "folder",

				Name: filepath.Base(relSlash),

				FolderID:    parentURI,
				Files:       []*FileNode{},
				Folders:     []*FolderNode{},
				DBInstances: []*DBInstanceNode{},
				Variables:   make(map[string]string),
			}

			foldersByID[folderURI] = folder

			// Attach to parent database instance or folder
			if parentDB, ok := dbInstancesByURI[normalizeDirURI(parentURI)]; ok {
				parentDB.Folders = append(parentDB.Folders, folder)
			} else if parentFolder != nil {
				parentFolder.Folders = append(parentFolder.Folders, folder)
			}

			// Load .env file for this folder if it exists
			_ = g.LoadFolderEnvFile(folder, fsCtx)

			return nil
		}

		// Regular file.
		name := d.Name()

		// Skip internal/config files that are not user-facing.
		if IsInternalWorkspaceFile(name) {
			return nil
		}

		parentURI := fsCtx.ParentURI(relSlash)
		parentFolder := g.getOrCreateParentFolder(foldersByID, parentURI, fsCtx)

		fileURI := fsCtx.URI(relSlash)

		var databases []DatabaseRef
		metaPath := cpath + ".metadata.json"
		if meta, metaErr := ReadFileMetadata(metaPath); metaErr == nil && len(meta.Databases) > 0 {
			databases = meta.Databases
		}

		node := &FileNode{
			ID:   fileURI,
			URI:  fileURI,
			Type: "file",

			Name: name,

			FolderID:       parentURI,
			Databases:      databases,
			QueryResults:   nil,
			PlanResults:    nil,
			ExplainResults: nil,
		}

		// Attach to parent database instance or folder
		if parentDB, ok := dbInstancesByURI[normalizeDirURI(parentURI)]; ok {
			parentDB.Files = append(parentDB.Files, node)
		} else if parentFolder != nil {
			parentFolder.Files = append(parentFolder.Files, node)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("walk workspace for graph: %w", err)
	}

	g.WorkspaceGraph.Folders = []*FolderNode{rootFolder}

	sortDBInstancesByName(rootFolder)

	return nil
}

func sortDBInstancesByName(folder *FolderNode) {
	slices.SortFunc(folder.DBInstances, func(a, b *DBInstanceNode) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	for _, sub := range folder.Folders {
		sortDBInstancesByName(sub)
	}
}

// getOrCreateParentFolder looks up a parent folder in the map, creating it if it doesn't exist.
// This handles cases where WalkDir might not have visited the parent yet.
func (g *Graph) getOrCreateParentFolder(foldersByID map[string]*FolderNode, parentURI string, fsCtx *WorkspaceFS) *FolderNode {
	if folder, ok := foldersByID[parentURI]; ok {
		return folder
	}
	// Parent folder doesn't exist yet, create a minimal one.
	parentRel, _ := fsCtx.Rel(parentURI)
	if parentRel == "" {
		parentRel = "."
	}
	folder := &FolderNode{
		ID:          parentURI,
		URI:         parentURI,
		Type:        "folder",
		Name:        filepath.Base(parentRel),
		FolderID:    "",
		Files:       []*FileNode{},
		Folders:     []*FolderNode{},
		DBInstances: []*DBInstanceNode{},
		Variables:   make(map[string]string),
	}
	foldersByID[parentURI] = folder
	return folder
}
