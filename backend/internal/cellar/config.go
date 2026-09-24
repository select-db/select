// Package cellar hosts managed SQLite databases: SQLite files served to the
// backend over its engine transport and replicated to a bucket.
package cellar

import (
	"fmt"
	"net/url"
	"strings"
)

// Mode is how the backend reaches its cellar.
type Mode int

const (
	// Disabled: managed databases are off and the backend behaves as without them.
	Disabled Mode = iota
	// Local: the cellar runs in the backend's own process.
	Local
	// Remote: the cellar is another process, reached at Config.URL.
	Remote
)

func (m Mode) String() string {
	switch m {
	case Local:
		return "local"
	case Remote:
		return "remote"
	default:
		return "disabled"
	}
}

// Config is the parsed CELLAR setting.
type Config struct {
	Mode Mode
	URL  string // set only for Remote
}

// Parse reads a CELLAR value: empty disables, "local" runs in-process, and an
// http or https URL points at a remote cellar. Anything else is an error, so a
// typo fails at startup instead of silently disabling the feature.
func Parse(v string) (Config, error) {
	v = strings.TrimSpace(v)
	switch v {
	case "":
		return Config{Mode: Disabled}, nil
	case "local":
		return Config{Mode: Local}, nil
	}
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Config{}, fmt.Errorf(`CELLAR=%q: want empty, "local" or an http(s) URL`, v)
	}
	return Config{Mode: Remote, URL: strings.TrimRight(v, "/")}, nil
}
