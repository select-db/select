//go:build windows && !server

package desktop

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

// windowsZoomFloor is the smallest factor Wails will apply on Windows.
const windowsZoomFloor = 1.0

// On Windows the zoom lives on WebView2's ICoreWebView2Controller, which Wails
// keeps private and which cannot be recovered from the window handle, so zoom
// has to go through Wails. Its wrapper clamps anything below 1.0, so zooming
// out past 100% is a no-op here until that clamp is lifted upstream; the
// applied factor is reported back so the frontend keeps its level in step with
// what the webview actually did.
func setZoom(window application.Window, factor float64) (float64, error) {
	if factor < windowsZoomFloor {
		factor = windowsZoomFloor
	}
	window.SetZoom(factor)

	return factor, nil
}

func getZoom(window application.Window) float64 {
	zoom := window.GetZoom()
	if zoom <= 0 {
		return 1
	}

	return zoom
}
