package main

import (
	"context"
	"embed"
	"log"
	"os"
	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	appcore "selectDb/internal/app"

	"github.com/selectDb/toolkit"
)

//go:embed all:frontend/build
var assets embed.FS

// Build-time vars, overridden via:
//
//	wails build -ldflags "-X main.apiURL=https://api.select-db.com -X main.appVersion=1.2.3 -X main.appEnv=production"
var (
	apiURL     = "http://localhost:8080"
	appVersion = "dev"
	appEnv     = "dev"
)

func main() {
	// Expose build-time vars to all packages that read them from env.
	// In a packaged desktop app there is no shell environment.
	_ = os.Setenv("API_URL", apiURL)
	_ = os.Setenv("APP_VERSION", appVersion)
	_ = os.Setenv("APP_ENV", appEnv)

	toolkit.StartPprofServer("localhost:6061")

	// Create an instance of the app structure
	app := appcore.NewApp()

	if len(os.Args) > 1 {
		var arg string
		if len(os.Args) > 2 {
			arg = os.Args[2]
		}
		_, err := app.Commands.Run(os.Args[1], arg)
		if err != nil {
			log.Fatalf("Error running command: %v", err)
		}
		return
	}

	var title string
	switch appEnv {
	case "staging":
		title = "Select (staging)"
	case "dev":
		title = "Select (dev)"
	default:
		title = "Select"
	}

	// Create application with options
	err := wails.Run(&options.App{
		EnableDefaultContextMenu: appEnv != "production",
		Title:                    title,
		Width:                    1024,
		MinWidth:                 450,
		Height:                   768,
		MinHeight:                450,
		LogLevel:                 5,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},

		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			IsZoomControlEnabled: true,
		},

		Mac: &mac.Options{
			Appearance:           mac.DefaultAppearance,
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			// TitleBarHidden gives frameless look but keeps native window so fullscreen/minimize work.
			TitleBar: mac.TitleBarHidden(),
		},

		Linux: &linux.Options{
			WindowIsTranslucent: false,
		},

		OnStartup: func(ctx context.Context) {
			app.Startup(ctx)
		},
		OnShutdown: func(ctx context.Context) {
			app.SqlLang.Close()
			app.Terminal.DestroyAll()
		},
		// On macOS use non-frameless + hidden titlebar so green
		// (fullscreen) and yellow (minimize) work.
		Frameless: runtime.GOOS != "darwin",
		Bind: []interface{}{
			app,
			app.Graph,
			app.System,
			app.GithubAuth,
			app.Git,
			app.Search,

			app.User,
			app.Workspace,
			app.Role,
			app.History,
			app.Server,

			app.Datasource,
			app.APIKey,

			app.DbClient,
			app.SqlLang,
			app.FSProvider,
			app.Updater,
			app.Terminal,
		},
	})

	if err != nil {
		log.Fatal("Wails boot error:", err.Error())
	}
}
