package system

import "os"

// GetAppEnv returns the build-time application environment ("production",
// "staging", "dev", ...). Set in main via -X main.appEnv and exported to the
// process env at startup. The frontend uses it to gate prod-only UI.
func (s *System) GetAppEnv() string {
	return os.Getenv("APP_ENV")
	// return "production"
}

// GetAppVersion returns the build-time application version. Set in main via
// -X main.appVersion and exported to the process env at startup; "dev" for a
// local build.
//
// The login screen shows it next to the server's own version. They are
// different numbers that happen to share a scheme on staging, where both are
// the commit count, and showing only the server's made a legitimate update
// prompt look like a bug: the one version on screen already matched the one
// being offered.
func (s *System) GetAppVersion() string {
	return os.Getenv("APP_VERSION")
}
