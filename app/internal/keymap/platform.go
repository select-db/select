package keymap

import "runtime"

// Platform is the operating system a keymap is resolved for. It decides one
// thing here -- what "secondary" means -- and is passed in rather than read
// from the environment so a keymap can be resolved for any platform in a test.
type Platform string

const (
	MacOS   Platform = "macos"
	Linux   Platform = "linux"
	Windows Platform = "windows"
)

// Current is the platform this binary runs on.
func Current() Platform {
	switch runtime.GOOS {
	case "darwin":
		return MacOS
	case "windows":
		return Windows
	default:
		return Linux
	}
}

// Secondary is the modifier this platform uses for its own shortcuts: the
// Command key on macOS, Control everywhere else. A binding written with
// "secondary" means "the shortcut key here", and this is where that is decided.
func (p Platform) Secondary() string {
	if p == MacOS {
		return "cmd"
	}
	return "ctrl"
}
