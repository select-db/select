package backend

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/joho/godotenv"
	"github.com/ngrok/sqlmw"
	"modernc.org/sqlite"

	commands "selectDb/backend/cmd/cli"
	"selectDb/backend/db"
	"selectDb/backend/db/generated"
	"selectDb/backend/db/syncer"
	"selectDb/backend/graph"
	"selectDb/backend/utils"

	auth "selectDb/backend/auth"
	git "selectDb/backend/git"
	search "selectDb/backend/search"
	apikey "selectDb/backend/src/apikey"
	datasource "selectDb/backend/src/datasource"
	db_client "selectDb/backend/src/db_client"
	fs_provider "selectDb/backend/src/fs_provider"
	history "selectDb/backend/src/history"
	role "selectDb/backend/src/role"
	server "selectDb/backend/src/server"
	"selectDb/backend/src/sqllang"
	user "selectDb/backend/src/user"
	workspace "selectDb/backend/src/workspace"
	system "selectDb/backend/system"
	"selectDb/backend/terminal"
	"selectDb/backend/updater"
)

type App struct {
	ctx context.Context

	Commands   *commands.Command
	InternalDb *db.InternalDb
	Syncer     *syncer.Syncer
	Graph      *graph.Graph
	System     *system.System
	GithubAuth *auth.GithubAuth

	Git    *git.Git
	Search *search.Search

	User       *user.User
	Workspace  *workspace.Workspace
	Role       *role.Role
	History    *history.History
	Datasource *datasource.Datasource
	APIKey     *apikey.APIKey

	DbClient   *db_client.DbClient
	SqlLang    *sqllang.SqlLang
	FSProvider *fs_provider.FSProvider
	Terminal   *terminal.Terminal
	Server     *server.Server
	Updater    *updater.Updater
}

var GlobalInterceptor = &db.SqlInterceptor{}

func NewApp() *App {
	// Best-effort: load .env if present. In production (packaged app), vars are
	// set by main.go via os.Setenv before NewApp is called.
	_ = godotenv.Load()

	sql.Register("sqlite3", &sqlite.Driver{})
	sql.Register("sqlite3-mw", sqlmw.Driver(&sqlite.Driver{}, GlobalInterceptor))

	if err := server.EnsureDefaultCurrentDomain(); err != nil {
		log.Fatal("Failed to ensure default server: ", err)
	}
	currentDomain, err := server.ReadCurrentDomain()
	if err != nil {
		log.Fatal("Failed to read current server: ", err)
	}
	if err := db.Init(currentDomain); err != nil {
		log.Fatal("Failed to initialize database: ", err)
	}

	// Use a live DBTX so Queries always use the current DB after a server switch.
	Queries := generated.New(db.NewLiveDB())
	Graph := graph.New(Queries)

	FSProvider := fs_provider.New()
	if currentDomain != "" {
		serverRoot, err := server.ServerRootPath(currentDomain)
		if err != nil {
			log.Fatal("Failed to get server root: ", err)
		}
		FSProvider.SetRoot(serverRoot)
	}

	Server := server.New(
		func(root string) {
			FSProvider.SetRoot(root)
			Graph.InvalidateWorkspaceGraph()
		},
		db.RunMigrationsAt,
		db.SwitchToServer,
	)

	Role := role.New(Queries)
	DbClient := db_client.New(Queries, Graph, FSProvider)
	SqlLang := sqllang.New(Graph, Queries, DbClient.GetMeta, DbClient.InspectStatement)
	Workspace := workspace.New(Queries, FSProvider)

	System := system.New(Queries, Graph, DbClient, FSProvider)
	Graph.AfterWorkspaceGraphBuild = func(ws *graph.WorkspaceNode) {
		System.LoadAllDatabaseSchemas(ws)
	}

	Git := git.New(Queries, FSProvider, Graph)
	Syncer := syncer.New(Queries, Graph, Workspace, Git)
	internalDb := db.New(Queries, Syncer)
	GlobalInterceptor.Db = internalDb

	return &App{
		InternalDb: internalDb,
		Syncer:     Syncer,
		Graph:      Graph,

		System:     System,
		GithubAuth: auth.New(Queries, Workspace, Syncer),
		Git:        Git,
		Search:     search.New(Graph),
		User:       user.New(Queries),
		Workspace:  Workspace,
		Role:       Role,
		History:    history.New(Queries),

		Datasource: datasource.New(Queries),
		APIKey:     apikey.New(Queries),

		DbClient:   DbClient,
		SqlLang:    SqlLang,
		FSProvider: FSProvider,
		Terminal:   terminal.New(Graph),
		Server:     Server,
		Updater:    updater.New(),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	a.InternalDb.Syncer.SetContext(ctx)
	a.Syncer.SetContext(ctx)
	a.Syncer.SwitchOrLogout = &switchOrLogoutHandler{app: a}
	a.Syncer.EmitRolesUpdated = func() {
		utils.DebouncedEventsEmit(a.ctx, "rolesUpdated", 100*time.Millisecond)
	}
	a.Syncer.EmitWorkspaceRepoChanged = func(res git.ReconcileResult) {
		if a.ctx == nil {
			return
		}
		utils.DebouncedEventsEmit(a.ctx, "workspaceGraphUpdated", 100*time.Millisecond, a.Graph.WorkspaceGraph)
		utils.DebouncedEventsEmit(a.ctx, "workspaceRepoChanged", 100*time.Millisecond, res)
	}
	a.Workspace.PullFunc = a.Syncer.Pull
	a.Workspace.ReloadHooks = &workspace.ReloadHooks{
		BuildWorkspaceGraph: a.Graph.RebuildWorkspaceGraph,
		EmitWorkspaceGraphUpdated: func() {
			utils.DebouncedEventsEmit(a.ctx, "workspaceGraphUpdated", 100*time.Millisecond, a.Graph.WorkspaceGraph)
		},
		RunSwitchOrLogout: a.Syncer.RunSwitchOrLogout,
		ReconcileGitRemote: func(workspaceID string) {
			ws, err := a.Git.Queries.GetWorkspaceByID(context.Background(), workspaceID)
			if err != nil {
				return
			}
			_, _ = a.Git.ReconcileWorkspaceRemote(workspaceID, ws.GitRemoteUrl.Ptr())
		},
	}
	a.GithubAuth.SetContext(ctx)
	a.Git.SetContext(ctx)
	a.Search.SetContext(ctx)

	a.DbClient.SetContext(ctx)

	a.Terminal.SetContext(ctx)

	a.System.SetContext(ctx)
	go a.System.WatchNetworkQuality()

	a.Updater.SetContext(ctx)
}
