package engine

import "fmt"

// ConfigError is a user-facing configuration error that is safe to return
// to the client. It does not contain network topology or internal details.
type ConfigError struct {
	Msg string
}

func (e *ConfigError) Error() string { return e.Msg }

func newConfigError(msg string) *ConfigError {
	return &ConfigError{Msg: msg}
}

func newConfigErrorf(format string, args ...any) *ConfigError {
	return &ConfigError{Msg: fmt.Sprintf(format, args...)}
}
