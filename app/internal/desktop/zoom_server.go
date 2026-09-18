//go:build server

package desktop

import "github.com/wailsapp/wails/v3/pkg/application"

// Server mode (`-tags server`) runs the app headless, serving the frontend over
// HTTP instead of embedding it in a webview. There is no webview to zoom, and
// the platform files that would reach one are cgo, so they are excluded from
// this build rather than linked against GUI libraries that need not be present.

func setZoom(_ application.Window, _ float64) (float64, error) {
	return 0, ErrZoomUnsupported
}

func getZoom(_ application.Window) float64 {
	return 1
}
