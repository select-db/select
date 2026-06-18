package system

import "os"

// GetAppEnv returns the build-time application environment ("production",
// "staging", "dev", ...). Set in main via -X main.appEnv and exported to the
// process env at startup. The frontend uses it to gate prod-only UI.
func (s *System) GetAppEnv() string {
	return os.Getenv("APP_ENV")
	// return "production"
}
