//go:build darwin

package desktop

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework WebKit

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

// Wails hands us the NSWindow; the webview is an ordinary view inside it.
static WKWebView* findWebView(NSView* view) {
	if (view == nil) {
		return nil;
	}
	if ([view isKindOfClass:[WKWebView class]]) {
		return (WKWebView*)view;
	}
	for (NSView* child in [view subviews]) {
		WKWebView* found = findWebView(child);
		if (found != nil) {
			return found;
		}
	}
	return nil;
}

// setPageZoom applies WebKit's page zoom — the reflowing zoom behind Safari's
// Cmd +/- — rather than WKWebView's magnification, which only scales the
// rendered page and leaves it pannable.
static bool setPageZoom(void* nsWindow, double zoom) {
	WKWebView* webView = findWebView([(NSWindow*)nsWindow contentView]);
	if (webView == nil || ![webView respondsToSelector:@selector(setPageZoom:)]) {
		return false;
	}
	[webView setPageZoom:(CGFloat)zoom];
	return true;
}

static double pageZoom(void* nsWindow) {
	WKWebView* webView = findWebView([(NSWindow*)nsWindow contentView]);
	if (webView == nil || ![webView respondsToSelector:@selector(pageZoom)]) {
		return 0;
	}
	return (double)[webView pageZoom];
}
*/
import "C"

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Wails v3 zooms macOS windows with WKWebView's magnification, so its own zoom
// API is not usable here: magnification scales the rendered page without
// re-laying it out, which leaves the window scrollable and misplaces anything
// positioned from client coordinates. pageZoom, public since macOS 11, is the
// browser-zoom equivalent, and unlike Wails' wrapper it accepts factors below
// 1.0.
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
