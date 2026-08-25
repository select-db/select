package desktop

import (
	"errors"
	"fmt"
)

// ErrZoomUnsupported is returned when the running platform cannot apply the
// requested zoom factor.
var ErrZoomUnsupported = errors.New("page zoom is not supported on this platform")

// SetZoom applies a page zoom factor to the webview and returns the factor that
// ended up being applied — which can differ from the requested one where the
// platform imposes a floor (see zoom_windows.go).
//
// This is page zoom, the kind a browser applies with Cmd/Ctrl +/-: the webview
// re-lays the document out at the new scale, so the frontend needs no
// compensation for it.
func SetZoom(factor float64) (float64, error) {
	if factor <= 0 {
		return 0, fmt.Errorf("invalid zoom factor %v", factor)
	}

	window := currentWindow()
	if window == nil {
		return 0, ErrNoApplication
	}

	return setZoom(window, factor)
}

// GetZoom returns the webview's current page zoom factor, or 1 when there is no
// window to read it from.
func GetZoom() float64 {
	window := currentWindow()
	if window == nil {
		return 1
	}

	return getZoom(window)
}
