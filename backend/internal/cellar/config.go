// Package cellar configures where managed SQLite databases are hosted.
package cellar

import (
	"fmt"
	"net/url"
	"strings"
)

// Config is the parsed CELLAR setting. The zero value means managed databases
// are off.
type Config struct {
	Local bool   // the cellar runs in the backend's own process
	URL   string // a remote cellar; empty unless remote
}

// Parse reads a CELLAR value: empty, "local", or an http(s) URL. Anything else
// is an error, so a typo cannot pass for "off".
func Parse(v string) (Config, error) {
	v = strings.TrimSpace(v)
	switch v {
	case "":
		return Config{}, nil
	case "local":
		return Config{Local: true}, nil
	}
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Config{}, fmt.Errorf(`CELLAR=%q: want empty, "local" or an http(s) URL`, v)
	}
	return Config{URL: strings.TrimRight(v, "/")}, nil
}
