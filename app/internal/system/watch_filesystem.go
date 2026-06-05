package system

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"selectDb/internal/db/generated"
	"selectDb/internal/db_client"
	"selectDb/internal/graph"
	"selectDb/internal/utils"

	"github.com/selectDb/dialect/engine"

	"github.com/fsnotify/fsnotify"
)

// Maps an fsnotify op to ("insert"|"delete", true), or ("", false) to ignore.
func classifyFSOp(op fsnotify.Op) (string, bool) {
	switch {
	case op&(fsnotify.Remove|fsnotify.Rename) != 0:
		return "delete", true
	case op&fsnotify.Create != 0:
		return "insert", true
	case op&fsnotify.Write != 0:
		return "", false
	default:
		return "", false
	}
}

// Stops any running watcher and starts a new one for workspaceID.
func (s *System) StartFileWatcher(workspaceID string) {
	s.mu.Lock()
	if s.fileWatcherCancel != nil {
		s.fileWatcherCancel()
	}
	ctx, cancel := context.WithCancel(s.ctx)
	s.fileWatcherCancel = cancel
	s.mu.Unlock()

	go s.watchWorkspace(ctx, workspaceID)
}

func (s *System) watchWorkspace(ctx context.Context, workspaceID string) {
	user, err := s.Queries.GetCurrentUser(ctx)
	if err != nil {
		return
	}

	fsCtx, err := graph.NewWorkspaceFS(workspaceID)
	if err != nil {
		// @todo handle error
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		// @todo handle error
		return
	}
	defer func() { _ = watcher.Close() }()

	// Watch all existing dirs; new ones are added dynamically on Create events.
	addWatches := func(root string) {
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if d.Name() == ".git" {
					return fs.SkipDir
				}
				_ = watcher.Add(p)
			}
			return nil
		})
	}

	addWatches(fsCtx.WorkspaceRoot)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			if strings.HasSuffix(event.Name, ".metadata.json") {
				s.handleMetadataEvent(event, user.ID, fsCtx)
				continue
			}

			if strings.HasSuffix(event.Name, "db.config.json") {
				s.handleDBConfigEvent(event, user.ID, fsCtx)
				continue
			}

			if filepath.Base(event.Name) == ".env" {
				s.handleEnvFileEvent(event, fsCtx)
			}

			if filepath.Base(event.Name) == graph.ThemeFileName {
				s.handleThemeFileEvent(event, fsCtx)
			}

			if filepath.Base(event.Name) == graph.ConfigFileName {
				s.handleConfigFileEvent(event, fsCtx)
			}

			if filepath.Base(event.Name) == graph.LintFileName {
				s.handleLintFileEvent(event, fsCtx)
			}

			// Track new directories for deeper-level events.
			if event.Op&fsnotify.Create != 0 {
				info, err := os.Stat(event.Name)
				if err == nil && info.IsDir() {
					_ = watcher.Add(event.Name)
				}
			}

			// Rename: full rebuild (fine-grained derivation is error-prone).
			if event.Op&fsnotify.Rename != 0 {
				s.rebuildGraphAndEmit()
				continue
			}

			if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove) != 0 {
				s.handleFSEvent(event, user.ID, fsCtx)
				utils.DebouncedEventsEmit(s.ctx, "gitDetailedStatusChanged", 200*time.Millisecond, nil)
			}
		case <-watcher.Errors:
			// @todo handle error
		case <-ctx.Done():
			return
		}
	}
}

// Rebuilds the workspace graph and emits workspaceGraphUpdated.
// Fallback for renames where incremental updates are unreliable.
func (s *System) rebuildGraphAndEmit() {
	if err := s.Graph.RebuildWorkspaceGraph(); err != nil {
		// @todo handle error
		return
	}
	wsGraph, err := s.Graph.GetWorkspaceGraph()
	if err != nil {
		return
	}
	utils.DebouncedEventsEmit(s.ctx, "workspaceGraphUpdated", 200*time.Millisecond, wsGraph)
}

// LoadAllDatabaseSchemas runs QuerySchema for each workspace DB instance (same as after other graph rebuilds).
func (s *System) LoadAllDatabaseSchemas(wsGraph *graph.WorkspaceNode) {
	if s.DbClient == nil || wsGraph == nil {
		return
	}
	for _, dbInstance := range wsGraph.DBInstances {
		dbID := dbInstance.ID
		go func(id string) {
			_ = s.DbClient.QuerySchema(db_client.QuerySchemaParams{
				DatabaseInstanceID: id,
				NoCache:            false,
			})
		}(dbID)
	}
}

// Emits a db_instance mutation when db.config.json changes.
func (s *System) handleDBConfigEvent(event fsnotify.Event, userID string, ctx *graph.WorkspaceFS) {
	if _, ok := ctx.Rel(event.Name); !ok {
		return
	}

	var op string
	if event.Op&fsnotify.Remove != 0 {
		op = "delete"
	} else if event.Op&(fsnotify.Create|fsnotify.Write) != 0 {
		op = "insert"
	} else {
		return
	}

	dirPath := filepath.Dir(event.Name)
	dirRel, ok := ctx.Rel(dirPath)
	if !ok {
		return
	}

	dbURI := ctx.URI(dirRel)

	if op == "delete" {
		s.emitMutation("db_instance", op, dbURI, nil, ctx.WorkspaceID, userID)
		return
	}

	cfg, err := graph.ReadFSDBConfig(event.Name)
	if err != nil {
		return
	}

	// Invalidate cached metadata so the next schema load uses the new config.
	engine.InvalidateMetadata(ctx.WorkspaceID, cfg.DSN)

	var sshConfig *graph.DBInstanceSSHConfig
	if cfg.SSH != nil {
		ssh := &graph.DBInstanceSSHConfig{
			Enabled:    cfg.SSH.Enabled,
			Host:       cfg.SSH.Host,
			Port:       cfg.SSH.Port,
			User:       cfg.SSH.User,
			AuthMethod: cfg.SSH.AuthMethod,
			Password:   cfg.SSH.Password,
			PrivateKey: cfg.SSH.PrivateKey,
		}
		if ssh.Enabled && ssh.Port == 0 {
			ssh.Port = 22
		}
		sshConfig = ssh
	}

	payload := graph.DBInstanceDTO{
		ID:          &cfg.ID,
		URI:         &dbURI,
		Name:        utils.Ptr(cfg.Name),
		DBType:      utils.Ptr(cfg.DbType),
		DSN:         utils.Ptr(cfg.DSN),
		Proxified:   utils.Ptr(cfg.Proxified),
		SSH:         sshConfig,
		FolderID:    utils.Ptr(ctx.ParentURI(dirRel)),
		WorkspaceID: utils.Ptr(ctx.WorkspaceID),
	}

	// If the db instance already exists in the graph, use "update" to
	// preserve cached schema children instead of replacing the node.
	if op == "insert" && s.Graph != nil && s.Graph.GetDBInstanceNodeByID(cfg.ID) != nil {
		op = "update"
	}

	s.emitMutation("db_instance", op, cfg.ID, payload, ctx.WorkspaceID, userID)

	// Introspect the db instance folder so files/folders appear in the UI (same as
	// processDirectoryEntry + scanFolderContents for other folders).
	s.scanFolderContents(dirPath, dbURI, userID, ctx)
}

// Reloads a folder's env variables when its .env file changes.
func (s *System) handleEnvFileEvent(event fsnotify.Event, ctx *graph.WorkspaceFS) {
	if _, ok := ctx.Rel(event.Name); !ok {
		return
	}

	folderPath := filepath.Dir(event.Name)
	folderRel, ok := ctx.Rel(folderPath)
	if !ok {
		return
	}

	folderURI := ctx.URI(folderRel)

	wsGraph, err := s.Graph.GetWorkspaceGraph()
	if err != nil || wsGraph == nil {
		return
	}

	nodes := graph.FindNodesByIds(wsGraph, []string{folderURI})
	if len(nodes) == 0 {
		return
	}

	folderNode, ok := nodes[0].(*graph.FolderNode)
	if !ok {
		return
	}

	if event.Op&fsnotify.Remove != 0 {
		folderNode.Variables = make(map[string]string)
	} else {
		wfs, err := graph.NewWorkspaceFS(wsGraph.ID)
		if err == nil {
			_ = s.Graph.LoadFolderEnvFile(folderNode, wfs)
		}
	}

	utils.DebouncedEventsEmit(s.ctx, "workspaceGraphUpdated", 200*time.Millisecond, wsGraph)
}

// Emits themeUpdated when the workspace .theme file changes.
// Always sends the merged state (defaults + user .theme); on remove or error, merged = defaults.
func (s *System) handleThemeFileEvent(event fsnotify.Event, ctx *graph.WorkspaceFS) {
	if _, ok := ctx.Rel(event.Name); !ok {
		return
	}

	themeVars, err := s.Graph.LoadWorkspaceTheme()
	if err != nil {
		themeVars = graph.LoadDefaultTheme()
	}
	utils.DebouncedEventsEmit(s.ctx, "themeUpdated", 100*time.Millisecond, themeVars)
}

// Emits configUpdated when the workspace .config file changes.
// Always sends the merged state (defaults + user .config); on remove or error, merged = defaults.
func (s *System) handleConfigFileEvent(event fsnotify.Event, ctx *graph.WorkspaceFS) {
	if _, ok := ctx.Rel(event.Name); !ok {
		return
	}

	configResponse, err := s.Graph.LoadWorkspaceConfig()
	if err != nil {
		return
	}
	utils.DebouncedEventsEmit(s.ctx, "configUpdated", 100*time.Millisecond, configResponse)
}

func (s *System) handleLintFileEvent(event fsnotify.Event, ctx *graph.WorkspaceFS) {
	if _, ok := ctx.Rel(event.Name); !ok {
		return
	}
	lintConfig, err := s.Graph.LoadWorkspaceLint()
	if err != nil {
		return
	}
	utils.DebouncedEventsEmit(s.ctx, "lintUpdated", 100*time.Millisecond, lintConfig)
}

// Emits a file update mutation when a .metadata.json sidecar changes.
func (s *System) handleMetadataEvent(event fsnotify.Event, userID string, ctx *graph.WorkspaceFS) {
	if _, ok := ctx.Rel(event.Name); !ok {
		return
	}

	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove) == 0 {
		return
	}

	if !strings.HasSuffix(event.Name, ".metadata.json") {
		return
	}

	filePath := strings.TrimSuffix(event.Name, ".metadata.json")
	fileRel, ok := ctx.Rel(filePath)
	if !ok {
		return
	}

	fileURI := ctx.URI(fileRel)

	var databases []graph.DatabaseRef
	if event.Op&(fsnotify.Remove) == 0 {
		meta, err := graph.ReadFileMetadata(event.Name)
		if err != nil {
			return
		}
		databases = meta.Databases
	}

	payload := graph.FileDTO{
		ID:        &fileURI,
		URI:       &fileURI,
		Databases: &databases,
	}

	s.emitMutation("file", "update", fileURI, payload, ctx.WorkspaceID, userID)
}

// Translates a filesystem event into incremental graph mutations.
func (s *System) handleFSEvent(event fsnotify.Event, userID string, ctx *graph.WorkspaceFS) {
	relSlash, ok := ctx.Rel(event.Name)
	if !ok {
		return
	}

	if graph.IsInternalWorkspacePath(relSlash) {
		return
	}

	op, ok := classifyFSOp(event.Op)
	if !ok {
		return
	}

	info, statErr := os.Stat(event.Name)
	isDir := statErr == nil && info.IsDir()

	uri := ctx.URI(relSlash)

	if op == "delete" {
		table := s.inferTableFromGraph(uri)
		if table == "" {
			if isDir || statErr != nil {
				table = "folder"
			} else {
				table = "file"
			}
		}
		s.emitMutation(table, op, uri, nil, ctx.WorkspaceID, userID)
		return
	}

	if isDir {
		parentURI := ctx.ParentURI(relSlash)
		s.processDirectoryEntry(event.Name, uri, parentURI, userID, ctx, op == "insert")
		return
	}

	s.processFileEntry(event.Name, uri, ctx.ParentURI(relSlash), userID, ctx, op)
}

// Resolves a node URI to its table name via the current graph.
func (s *System) inferTableFromGraph(id string) string {
	ws, err := s.Graph.GetWorkspaceGraph()
	if err != nil || ws == nil {
		return ""
	}

	nodes := graph.FindNodesByIds(ws, []string{id})
	if len(nodes) == 0 {
		return ""
	}

	switch nodes[0].(type) {
	case *graph.FileNode:
		return "file"
	case *graph.FolderNode:
		return "folder"
	case *graph.DBInstanceNode:
		return "db_instance"
	default:
		return ""
	}
}

// Emits a file mutation, skipping internal workspace files.
func (s *System) processFileEntry(filePath, fileURI, parentURI string, userID string, ctx *graph.WorkspaceFS, op string) {
	name := filepath.Base(filePath)
	if graph.IsInternalWorkspaceFile(name) {
		return
	}

	payload := graph.FileDTO{
		ID:       &fileURI,
		URI:      &fileURI,
		Name:     utils.Ptr(name),
		FolderID: utils.Ptr(parentURI),
	}
	s.emitMutation("file", op, fileURI, payload, ctx.WorkspaceID, userID)
}

// Handles the directory if it contains db.config.json.
func (s *System) checkAndHandleDBInstance(dirPath string, userID string, ctx *graph.WorkspaceFS) bool {
	if !graph.CheckIsDBInstance(dirPath) {
		return false
	}
	s.handleDBConfigEvent(fsnotify.Event{Name: filepath.Join(dirPath, "db.config.json"), Op: fsnotify.Create}, userID, ctx)
	return true
}

// Emits a folder mutation and optionally scans its contents recursively.
func (s *System) processDirectoryEntry(dirPath, dirURI, parentURI string, userID string, ctx *graph.WorkspaceFS, scanContents bool) {
	if s.checkAndHandleDBInstance(dirPath, userID, ctx) {
		return
	}

	relSlash, _ := ctx.Rel(dirPath)
	payload := graph.FolderDTO{
		ID:       &dirURI,
		URI:      &dirURI,
		Name:     utils.Ptr(filepath.Base(relSlash)),
		FolderID: utils.Ptr(parentURI),
	}
	s.emitMutation("folder", "insert", dirURI, payload, ctx.WorkspaceID, userID)

	// Scan contents to catch files restored e.g. via git.
	if scanContents {
		s.scanFolderContents(dirPath, dirURI, userID, ctx)
	}
}

// Recursively emits insert mutations for all children of a new folder.
func (s *System) scanFolderContents(folderPath, folderURI string, userID string, ctx *graph.WorkspaceFS) {
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return
	}

	for _, entry := range entries {
		childPath := filepath.Join(folderPath, entry.Name())

		relSlash, ok := ctx.Rel(childPath)
		if !ok || graph.IsInternalWorkspacePath(relSlash) {
			continue
		}

		childURI := ctx.URI(relSlash)

		if entry.IsDir() {
			s.processDirectoryEntry(childPath, childURI, folderURI, userID, ctx, true)
		} else {
			s.processFileEntry(childPath, childURI, folderURI, userID, ctx, "insert")
		}
	}
}

// Sends a MutationCommit to the graph for incremental updates.
func (s *System) emitMutation(table, op, id string, payload interface{}, workspaceID, userID string) {
	// Graph.Mutate requires a non-nil payload even for deletes.
	if op == "delete" && payload == nil {
		payload = map[string]interface{}{}
	}

	commit := generated.MutationCommit{
		Operation:   op,
		TableName:   table,
		ObjectID:    id,
		Payload:     payload,
		UserID:      userID,
		WorkspaceID: workspaceID,
	}

	// emitHook short-circuits in tests.
	if s.emitHook != nil {
		s.emitHook(commit)
		return
	}

	if s.Graph != nil {
		_ = s.Graph.Mutate(s.ctx, commit)
	}
}
