package server

import "os"

// DefaultDomainForEnv returns the default server domain for the current APP_ENV, or empty if none.
func DefaultDomainForEnv() string {
	switch os.Getenv("APP_ENV") {
	case "staging":
		return "staging-api.select-db.com"
	case "production", "prod":
		return "api.select-db.com"
	case "dev", "development", "default":
		return "localhost:8080"
	default:
		return ""
	}
}

// EnsureDefaultCurrentDomain ensures a current server is set when appropriate.
// If the current-server file is empty and the env has a default (staging/prod), creates
// the default server folder and writes the domain to the file.
func EnsureDefaultCurrentDomain() error {
	domain, err := ReadCurrentDomain()
	if err != nil {
		return err
	}
	if domain != "" {
		return nil
	}
	defaultDomain := DefaultDomainForEnv()
	if defaultDomain == "" {
		return nil
	}
	serverDir, err := ServerRootPath(defaultDomain)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(serverDir, 0o700); err != nil {
		return err
	}
	return WriteCurrentDomain(defaultDomain)
}
