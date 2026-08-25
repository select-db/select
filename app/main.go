package main

import (
	"embed"
	"log"
	"log/slog"
	"os"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"

	appcore "selectDb/internal/app"

	"github.com/selectDb/toolkit"
)

//go:embed all:frontend/build
var assets embed.FS

// Build-time vars, overridden via:
//
//	wails3 build -ldflags "-X main.appVersion=1.2.3 -X main.appEnv=production"
var (
	appVersion = "dev"
	appEnv     = "dev"
)

func main() {
	// Expose build-time vars to all packages that read them from env.
	// In a packaged desktop app there is no shell environment.
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

	isProduction := appEnv == "production"

	// Create application with options. Services replace v2's `Bind` list: each
	// one is exposed to the frontend through the generated bindings, and the
	// app service drives startup via its ServiceStartup method.
	wailsApp := application.New(application.Options{
		Name:        "Select",
		Description: "Select database client",
		LogLevel:    slog.LevelError,
		Services: []application.Service{
			application.NewService(app),
			application.NewService(app.Graph),
			application.NewService(app.System),
			application.NewService(app.GithubAuth),
			application.NewService(app.Git),
			application.NewService(app.Search),

			application.NewService(app.User),
			application.NewService(app.Workspace),
			application.NewService(app.Role),
			application.NewService(app.Group),
			application.NewService(app.History),
			application.NewService(app.Server),

			application.NewService(app.Datasource),
			application.NewService(app.APIKey),

			application.NewService(app.DbClient),
			application.NewService(app.SqlLang),
			application.NewService(app.FSProvider),
			application.NewService(app.Updater),
			application.NewService(app.Terminal),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		OnShutdown: func() {
			app.SqlLang.Close()
			app.Terminal.DestroyAll()
		},
	})

	wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     title,
		Width:     1024,
		MinWidth:  450,
		Height:    768,
		MinHeight: 450,
		URL:       "/",

		// On macOS use non-frameless + hidden titlebar so green
		// (fullscreen) and yellow (minimize) work.
		Frameless: runtime.GOOS != "darwin",

		// The zoom ladder lives in the frontend (see $lib/wails/zoom), so the
		// webview's own Ctrl+wheel accelerators stay off: they would move the
		// zoom factor without the app knowing about it.
		ZoomControlEnabled: false,

		DevToolsEnabled:            !isProduction,
		DefaultContextMenuDisabled: isProduction,

		Mac: application.MacWindow{
			Appearance: application.DefaultAppearance,
			Backdrop:   application.MacBackdropTranslucent,
			// TitleBarHidden gives frameless look but keeps native window so
			// fullscreen/minimize work.
			TitleBar: application.MacTitleBarHidden,
		},

		Linux: application.LinuxWindow{
			WindowIsTranslucent: false,
		},
	})

	if err := wailsApp.Run(); err != nil {
		log.Fatal("Wails boot error:", err.Error())
	}
}
