// Package desktop wraps the Wails v3 application runtime.
//
// Wails v3 drops the context argument that v2 required on every runtime call
// and exposes the running application through a package-level singleton. That
// singleton is nil whenever no window is running — CLI invocations
// (`select <command>`) and unit tests — so every call is guarded here instead
// of at each call site.
package desktop

import (
	"errors"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Emit sends a custom event to the frontend. It is a no-op when the desktop
// application is not running.
func Emit(eventName string, data ...any) {
	app := application.Get()
	if app == nil {
		return
	}
	app.Event.Emit(eventName, data...)
}

// Quit stops the application, closing all windows.
func Quit() {
	app := application.Get()
	if app == nil {
		return
	}
	app.Quit()
}

// MinimiseWindow minimises the current window.
func MinimiseWindow() {
	if window := currentWindow(); window != nil {
		window.Minimise()
	}
}

// ToggleMaximiseWindow maximises the current window, or restores it if it is
// already maximised.
func ToggleMaximiseWindow() {
	if window := currentWindow(); window != nil {
		window.ToggleMaximise()
	}
}

func currentWindow() application.Window {
	app := application.Get()
	if app == nil {
		return nil
	}
	return app.Window.Current()
}

// ErrNoApplication is returned by the dialog helpers when they are called
// without a running desktop application.
var ErrNoApplication = errors.New("no desktop application is running")

// FileFilter describes one entry of a file dialog's type filter.
type FileFilter struct {
	DisplayName string // e.g. "CSV (*.csv)"
	Pattern     string // semicolon separated extensions, e.g. "*.csv"
}

// OpenFile shows a native file picker and returns the selected path. The
// returned path is empty when the user cancels.
func OpenFile(title, defaultDirectory string, showHiddenFiles bool) (string, error) {
	app := application.Get()
	if app == nil {
		return "", ErrNoApplication
	}

	dialog := app.Dialog.OpenFile().
		SetTitle(title).
		CanChooseFiles(true).
		ShowHiddenFiles(showHiddenFiles)
	if defaultDirectory != "" {
		dialog.SetDirectory(defaultDirectory)
	}

	return dialog.PromptForSingleSelection()
}

// SaveFile shows a native save panel and returns the chosen path. The returned
// path is empty when the user cancels.
func SaveFile(title, defaultFilename string, filters []FileFilter) (string, error) {
	app := application.Get()
	if app == nil {
		return "", ErrNoApplication
	}

	// SaveFileDialogStruct has no SetTitle builder method, so the title goes
	// through the options struct.
	dialog := app.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		Title:    title,
		Filename: defaultFilename,
	})
	for _, filter := range filters {
		dialog.AddFilter(filter.DisplayName, filter.Pattern)
	}

	return dialog.PromptForSingleSelection()
}
