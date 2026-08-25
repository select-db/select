//go:build linux && !gtk3

package desktop

/*
#cgo pkg-config: gtk4 webkitgtk-6.0

#include <gtk/gtk.h>
#include <webkit/webkit.h>

// Wails hands us the GtkWindow; the webview is a widget somewhere below it.
// GTK4 dropped GtkContainer, so children are walked through the widget itself.
static WebKitWebView* findWebView(GtkWidget* widget) {
	if (widget == NULL) {
		return NULL;
	}
	if (WEBKIT_IS_WEB_VIEW(widget)) {
		return WEBKIT_WEB_VIEW(widget);
	}

	for (GtkWidget* child = gtk_widget_get_first_child(widget);
	     child != NULL;
	     child = gtk_widget_get_next_sibling(child)) {
		WebKitWebView* found = findWebView(child);
		if (found != NULL) {
			return found;
		}
	}

	return NULL;
}

static bool setPageZoom(void* gtkWindow, double zoom) {
	WebKitWebView* webView = findWebView(GTK_WIDGET(gtkWindow));
	if (webView == NULL) {
		return false;
	}
	webkit_web_view_set_zoom_level(webView, (gdouble)zoom);
	return true;
}

static double pageZoom(void* gtkWindow) {
	WebKitWebView* webView = findWebView(GTK_WIDGET(gtkWindow));
	if (webView == NULL) {
		return 0;
	}
	return (double)webkit_web_view_get_zoom_level(webView);
}
*/
import "C"

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

// WebKitGTK's zoom level is already page zoom, but Wails' wrapper around it
// refuses factors below 1.0, so the webview is driven directly.
func setZoom(window application.Window, factor float64) (float64, error) {
	applied := application.InvokeSyncWithResult(func() bool {
		return bool(C.setPageZoom(window.NativeWindow(), C.double(factor)))
	})
	if !applied {
		return 0, ErrZoomUnsupported
	}

	return factor, nil
}

func getZoom(window application.Window) float64 {
	zoom := application.InvokeSyncWithResult(func() float64 {
		return float64(C.pageZoom(window.NativeWindow()))
	})
	if zoom <= 0 {
		return 1
	}

	return zoom
}
