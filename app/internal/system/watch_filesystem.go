package system

import (
	"context"
	"log"
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
	//
	// One watch per directory, so a workspace with more folders than the
	// platform's watch limit (inotify's max_user_watches, commonly 8192) gets
	// refusals partway through the walk. Those folders then go silent: no
	// mutation, no graph update, nothing in the UI to say why. The count is
	// logged once rather than per directory.
	addWatches := func(root string) {
		refused := 0
		if err := watcher.Add(root); err != nil {
			refused++
		}
		_ = fsCtx.WalkFrom(root, func(entry graph.Entry) error {
			if entry.IsDir() {
				if addErr := watcher.Add(entry.Path); addErr != nil {
					refused++
				}
			}
			return nil
		})
		if refused > 0 {
			log.Printf("[watcher] %d directories under %s could not be watched (platform watch limit?); changes there will not reach the workspace graph", refused, root)
		}
	}

	addWatches(fsCtx.WorkspaceRoot)

	// Also watch the per-user config dir so edits to the personal .theme /
	// .config hot-reload exactly like workspace files. These files live outside
	// the workspace, so their events are routed straight to the theme/config
	// reload handlers (never to the workspace graph).
	userConfigDir, _ := graph.UserConfigDir()
	if userConfigDir != "" {
		_ = watcher.Add(userConfigDir)
	}

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
			//
			// The whole subtree, not just this level: a directory can arrive
			// with children already in it — mkdir -p, a checkout, an unzip, a
			// clone — and those children raise no Create of their own, so
			// watching only the directory named here leaves them silent.
			if event.Op&fsnotify.Create != 0 {
				info, err := os.Stat(event.Name)
				if err == nil && info.IsDir() {
					addWatches(event.Name)
				}
			}

			// Rename: full rebuild (fine-grained derivation is error-prone).
			if event.Op&fsnotify.Rename != 0 {
				// A watch is registered against a path. A renamed directory
				// keeps its watch, so its children keep arriving under the old
				// name — and land in the graph under a folder that no longer
				// exists, which is to say nowhere. Re-walking registers the new
				// names; adding a directory that is already watched is a no-op.
				addWatches(fsCtx.WorkspaceRoot)
				s.rebuildGraphAndEmit()
				continue
			}

			if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove) != 0 {
				s.handleFSEvent(event, user.ID, fsCtx)
				utils.DebouncedEventsEmit("gitDetailedStatusChanged", 200*time.Millisecond, nil)
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
	utils.DebouncedEventsEmit("workspaceGraphUpdated", 200*time.Millisecond, wsGraph)
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

	sshConfig := graph.SSHConfigFromFS(cfg.SSH)

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

	folderNode := s.Graph.GetFolderNodeByID(folderURI)
	if folderNode == nil {
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

	utils.DebouncedEventsEmit("workspaceGraphUpdated", 200*time.Millisecond, wsGraph)
}

// Emits themeUpdated when the per-user .theme file changes.
// Always sends the merged state (defaults + user .theme); on remove or error, merged = defaults.
// The .theme file lives in the per-user config dir, so events come from there
// rather than the workspace root.
func (s *System) handleThemeFileEvent(event fsnotify.Event, ctx *graph.WorkspaceFS) {
	themeVars, err := s.Graph.LoadWorkspaceTheme()
	if err != nil {
		themeVars = graph.LoadDefaultTheme()
	}
	utils.DebouncedEventsEmit("themeUpdated", 100*time.Millisecond, themeVars)
}

// Emits configUpdated when the per-user .config file changes. Always sends the
// merged state (defaults + user keybindings/snippets); on remove or error,
// merged = defaults.
func (s *System) handleConfigFileEvent(event fsnotify.Event, ctx *graph.WorkspaceFS) {
	configResponse, err := s.Graph.LoadConfig()
	if err != nil {
		return
	}
	utils.DebouncedEventsEmit("configUpdated", 100*time.Millisecond, configResponse)
}

func (s *System) handleLintFileEvent(event fsnotify.Event, ctx *graph.WorkspaceFS) {
	if _, ok := ctx.Rel(event.Name); !ok {
		return
	}
	lintConfig, err := s.Graph.LoadWorkspaceLint()
	if err != nil {
		return
	}
	utils.DebouncedEventsEmit("lintUpdated", 100*time.Millisecond, lintConfig)
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
		// The path is gone, so the graph is all that says what it was: it holds
		// every folder it has seen, but a file only once its folder has been
		// opened. An unknown URI is taken for a file — the graph can miss a
		// folder too (made while the app was down, or never watched), and that
		// way round costs a tab close for a URI with no tab, where the other
		// leaves a tab open on a file that is gone.
		table := s.inferTableFromGraph(uri)
		if table == "" {
			if isDir {
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
	if s.Graph == nil {
		return ""
	}
	return s.Graph.NodeKind(id)
}

// Emits a file mutation, skipping internal workspace files and files whose
// folder has not been opened yet — an unresolved folder reads its files when it
// is opened, so putting one file in it now would only make it look resolved.
func (s *System) processFileEntry(filePath, fileURI, parentURI string, userID string, ctx *graph.WorkspaceFS, op string) {
	name := filepath.Base(filePath)
	if graph.IsInternalWorkspaceFile(name) {
		return
	}

	if !s.parentAcceptsFiles(parentURI) {
		return
	}

	payload := graph.FileDTOFromNode(graph.FileNodeFromDisk(filePath, fileURI, parentURI))
	s.emitMutation("file", op, fileURI, payload, ctx.WorkspaceID, userID)
}

// Reports whether a file event's parent is a container the graph tracks files
// for: a db instance directory, or a resolved folder. A parent the graph does
// not know — a folder whose own insert is still in flight — is accepted, so its
// files are not lost.
func (s *System) parentAcceptsFiles(parentURI string) bool {
	if s.Graph == nil {
		return true
	}

	parent := s.Graph.GetFolderNodeByID(parentURI)
	if parent == nil {
		return true
	}
	return parent.Resolved
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

	// Scan contents to catch what arrived with the directory rather than after
	// it: a checkout, a clone, a mkdir -p. Its children raise no event of their
	// own, and the graph holds every folder, so the folders in there have to be
	// taken now. The files in there are filtered by processFileEntry, which
	// drops the ones whose folder nobody has opened.
	if scanContents {
		s.scanFolderContents(dirPath, dirURI, userID, ctx)
	}
}

// Recursively emits insert mutations for all children of a new folder.
func (s *System) scanFolderContents(folderPath, folderURI string, userID string, ctx *graph.WorkspaceFS) {
	_ = ctx.ReadDir(folderPath, func(entry graph.Entry) error {
		childURI := entry.URI()

		if entry.IsDir() {
			s.processDirectoryEntry(entry.Path, childURI, folderURI, userID, ctx, true)
		} else {
			s.processFileEntry(entry.Path, childURI, folderURI, userID, ctx, "insert")
		}
		return nil
	})
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
