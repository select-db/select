package graph

// A build lays out a workspace's folders and db instances but not its files. A
// folder reads its files the first time it is opened and records that in
// FolderNode.Resolved; a lookup by ID resolves the folders along the path first,
// so callers never have to know whether a folder has been opened.

import (
	"fmt"
	"strings"
	"time"

	"selectDb/internal/utils"
)

// materializeFiles reads dirPath and attaches a node for every user-facing file
// the container does not already hold. Callers hold g.mu.
func (g *Graph) materializeFiles(container Node, dirPath string, fsCtx *WorkspaceFS) error {
	err := fsCtx.ReadDir(dirPath, func(entry Entry) error {
		if entry.IsDir() {
			return nil
		}

		fileURI := entry.URI()
		if g.lookup(fileURI) != nil {
			return nil
		}

		node := FileNodeFromDisk(entry.Path, fileURI, entry.ParentURI())
		container.AddChild(node)
		g.index.add(node)
		return nil
	})
	if err != nil {
		return fmt.Errorf("read folder %s: %w", dirPath, err)
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
// folder. A resolved folder is a no-op, an ID that names anything else returns
// nil, and the folder is returned so a caller that acts on its contents does not
// have to wait for the event.
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

// resolvedFolderURIs lists the folders whose files have been read, so a build
// can read them again. Callers hold g.mu.
func (g *Graph) resolvedFolderURIs() []string {
	uris := make([]string, 0, len(g.index))
	for id, node := range g.index {
		if folder, ok := node.(*FolderNode); ok && folder.Resolved && folder.URI == id {
			uris = append(uris, id)
		}
	}
	return uris
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

// FileDatabases returns the databases a file is bound to: from the node when
// the file's folder has been resolved, from the file's sidecar when it has not,
// which answers without resolving the folder.
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
