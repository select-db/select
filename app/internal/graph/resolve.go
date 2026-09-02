package graph

// A build lays out a workspace's folders and db instances but not its files.
// Files are the bulk of it — 20k files sit in about 1k folders — and most
// belong to folders nobody opens in a session, yet each one costs a sidecar
// read, a node, and a place in every graph payload sent to the frontend.
//
// So a folder reads its files the first time it is opened, and remembers that
// in FolderNode.Resolved. Lookups for a file nobody browsed to — a link, a tab
// restored from the last session — resolve the folders along its path first, so
// callers never have to know whether a folder has been opened.

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

// ResolveFolder reads a folder's files, emits the updated graph and returns the
// folder. The frontend calls it when a folder is opened; an already resolved
// folder is a no-op and anything that is not a folder returns nil, so callers
// do not have to check first.
//
// The folder comes back rather than only arriving with the graph event because
// a caller acting on what is in it — naming a new file so it does not land on
// an existing one — needs it before the next render.
func (g *Graph) ResolveFolder(folderURI string) (*FolderNode, error) {
	g.mu.Lock()

	folder, ok := g.lookup(folderURI).(*FolderNode)
	if !ok {
		g.mu.Unlock()
		return nil, nil
	}

	fsCtx, err := g.workspaceFS()
	if err != nil {
		g.mu.Unlock()
		return nil, err
	}

	resolved, err := g.resolveFolder(folder, fsCtx)
	wsGraph := g.WorkspaceGraph
	g.mu.Unlock()

	if err != nil {
		return nil, err
	}
	if resolved {
		utils.DebouncedEventsEmit("workspaceGraphUpdated", 100*time.Millisecond, wsGraph)
	}

	return folder, nil
}

// resolveAlongPath resolves every folder between the workspace root and the
// given URI, so a node nobody has browsed to can still be found by ID. Callers
// hold g.mu.
func (g *Graph) resolveAlongPath(uri string, fsCtx *WorkspaceFS) {
	rel, ok := strings.CutPrefix(uri, fsCtx.RootURI)
	if !ok {
		return
	}

	// The last segment is the node itself, not a folder to descend into.
	segments := strings.Split(strings.Trim(rel, "/"), "/")
	current := fsCtx.RootURI
	for _, segment := range segments[:max(0, len(segments)-1)] {
		g.resolveFolderURI(current, fsCtx)
		current += "/" + segment
	}
	g.resolveFolderURI(current, fsCtx)
}

// resolveFolderURI reads a folder's files when the URI names one, and lets a
// URI that names anything else pass. Callers hold g.mu.
func (g *Graph) resolveFolderURI(uri string, fsCtx *WorkspaceFS) {
	if folder, ok := g.lookup(uri).(*FolderNode); ok {
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
