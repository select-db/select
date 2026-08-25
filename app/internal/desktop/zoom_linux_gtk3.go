//go:build linux && gtk3

package desktop

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1

#include <gtk/gtk.h>
#include <webkit2/webkit2.h>

// Wails hands us the GtkWindow; the webview is a widget somewhere below it.
static WebKitWebView* findWebView(GtkWidget* widget) {
	if (widget == NULL) {
		return NULL;
	}
	if (WEBKIT_IS_WEB_VIEW(widget)) {
		return WEBKIT_WEB_VIEW(widget);
	}
	if (!GTK_IS_CONTAINER(widget)) {
		return NULL;
	}

	WebKitWebView* found = NULL;
	GList* children = gtk_container_get_children(GTK_CONTAINER(widget));
	for (GList* child = children; child != NULL && found == NULL; child = child->next) {
		found = findWebView(GTK_WIDGET(child->data));
	}
	g_list_free(children);

	return found;
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
