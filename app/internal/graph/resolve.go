package graph

// A build lays out the folder skeleton of a workspace — every directory, every
// db instance — but not its files. Files are the bulk of a workspace (a project
// with 20k files has ~1k folders) and most of them belong to folders the user
// never opens in a session, so materializing them all costs a walk that reads a
// sidecar per file, a node per file, and a copy of all of it in every graph
// payload sent to the frontend.
//
// Instead a folder's files are read the first time it is opened, and the folder
// remembers that with FolderNode.Resolved. Lookups that need a file the user
// has not browsed to — opening one from a link, or from a previous session's
// tabs — resolve the folders along its path first, so callers never have to
// know whether a folder has been opened yet.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"selectDb/internal/utils"
)

// newFSFileNode builds the file node for a directory entry, reading the sidecar
// metadata that binds the file to its databases.
func newFSFileNode(filePath, fileURI, parentURI, name string) *FileNode {
	var databases []DatabaseRef
	if meta, err := ReadFileMetadata(filePath + ".metadata.json"); err == nil && len(meta.Databases) > 0 {
		databases = meta.Databases
	}

	return &FileNode{
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
}

// materializeFiles reads dirPath and attaches a node for every user-facing file
// the container does not already hold. Callers hold g.mu.
func (g *Graph) materializeFiles(container Node, dirPath string, fsCtx *WorkspaceFS) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("read folder %s: %w", dirPath, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if IsInternalWorkspaceFile(name) {
			continue
		}

		childPath := filepath.Join(dirPath, name)
		relSlash, ok := fsCtx.Rel(childPath)
		if !ok || IsInternalWorkspacePath(relSlash) {
			continue
		}

		fileURI := fsCtx.URI(relSlash)
		if g.lookup(fileURI) != nil {
			continue
		}

		node := newFSFileNode(childPath, fileURI, fsCtx.ParentURI(relSlash), name)
		container.AddChild(node)
		g.index.add(node)
	}

	return nil
}

// resolveFolder materializes a folder's files. It is a no-op for a folder that
// is already resolved, and reports whether it did any work. Callers hold g.mu.
func (g *Graph) resolveFolder(folder *FolderNode, fsCtx *WorkspaceFS) (bool, error) {
	if folder == nil || folder.Resolved {
		return false, nil
	}

	dirPath, ok := fsCtx.Path(folder.URI)
	if !ok {
		return false, fmt.Errorf("folder %s is not in this workspace", folder.URI)
	}

	if err := g.materializeFiles(folder, dirPath, fsCtx); err != nil {
		return false, err
	}

	folder.Resolved = true
	ensureFolderArrays(folder)

	return true, nil
}

// workspaceFS builds the path context for the loaded workspace. Callers hold g.mu.
func (g *Graph) workspaceFS() (*WorkspaceFS, error) {
	if g.WorkspaceGraph == nil {
		return nil, fmt.Errorf("no workspace graph")
	}
	return NewWorkspaceFS(g.WorkspaceGraph.ID)
}

// ResolveFolder reads the files of the folder with the given URI and emits the
// updated graph. The frontend calls it when a folder is opened; it is a no-op
// for an unknown URI, a folder that is already resolved, or a node that is not
// a folder, so callers do not have to check first.
func (g *Graph) ResolveFolder(folderURI string) error {
	g.mu.Lock()

	folder, ok := g.lookup(folderURI).(*FolderNode)
	if !ok {
		g.mu.Unlock()
		return nil
	}

	fsCtx, err := g.workspaceFS()
	if err != nil {
		g.mu.Unlock()
		return err
	}

	resolved, err := g.resolveFolder(folder, fsCtx)
	wsGraph := g.WorkspaceGraph
	g.mu.Unlock()

	if err != nil {
		return err
	}
	if !resolved {
		return nil
	}

	utils.DebouncedEventsEmit("workspaceGraphUpdated", 100*time.Millisecond, wsGraph)
	return nil
}

// resolveAlongPath resolves every folder between the workspace root and the
// given URI, so that a node nobody has browsed to can still be found by ID.
// Callers hold g.mu.
func (g *Graph) resolveAlongPath(uri string, fsCtx *WorkspaceFS) {
	rel, ok := strings.CutPrefix(uri, fsCtx.RootURI)
	if !ok {
		return
	}

	segments := strings.Split(strings.Trim(rel, "/"), "/")
	current := fsCtx.RootURI

	// The last segment is the node itself, not a folder to descend into.
	for _, segment := range segments[:max(0, len(segments)-1)] {
		if folder, isFolder := g.lookup(current).(*FolderNode); isFolder {
			if _, err := g.resolveFolder(folder, fsCtx); err != nil {
				return
			}
		}
		current += "/" + segment
	}

	if folder, isFolder := g.lookup(current).(*FolderNode); isFolder {
		_, _ = g.resolveFolder(folder, fsCtx)
	}
}

// nodeForURI returns the node a URI addresses, reading the folders along its
// path first when they have not been read yet.
func (g *Graph) nodeForURI(uri string) Node {
	g.mu.Lock()
	defer g.mu.Unlock()

	if node := g.lookup(uri); node != nil {
		return node
	}

	fsCtx, err := g.workspaceFS()
	if err != nil {
		return nil
	}
	g.resolveAlongPath(uri, fsCtx)

	return g.lookup(uri)
}

// folderWithFiles returns the folder with the given ID, having read its files
// from disk. For callers that need what is *in* a folder rather than where it
// sits, which an unresolved folder cannot answer.
func (g *Graph) folderWithFiles(folderID string) *FolderNode {
	g.mu.Lock()
	defer g.mu.Unlock()

	folder, ok := g.lookup(folderID).(*FolderNode)
	if !ok {
		return nil
	}

	if fsCtx, err := g.workspaceFS(); err == nil {
		_, _ = g.resolveFolder(folder, fsCtx)
	}

	return folder
}

// FileDatabases returns the databases a file is bound to. It answers from the
// graph when the file's folder has been resolved, and from the file's sidecar
// when it has not, so callers get the same answer either way without pulling
// the whole folder into memory.
func (g *Graph) FileDatabases(fileURI string) []DatabaseRef {
	g.mu.RLock()
	node, _ := g.lookup(fileURI).(*FileNode)
	workspaceID := ""
	if g.WorkspaceGraph != nil {
		workspaceID = g.WorkspaceGraph.ID
	}
	g.mu.RUnlock()

	if node != nil {
		return node.Databases
	}
	if workspaceID == "" {
		return nil
	}

	fsCtx, err := NewWorkspaceFS(workspaceID)
	if err != nil {
		return nil
	}
	path, ok := fsCtx.Path(fileURI)
	if !ok {
		return nil
	}
	meta, err := ReadFileMetadata(path + ".metadata.json")
	if err != nil {
		return nil
	}
	return meta.Databases
}
