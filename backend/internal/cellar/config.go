// Package cellar configures where managed SQLite databases are hosted.
package cellar

import (
	"fmt"
	"net/url"
	"strings"
)

// Local is the CELLAR value that runs the cellar in the backend's own process.
const Local = "local"

// Parse checks a CELLAR value and returns it normalized: "" (managed databases
// off), Local, or an http(s) URL. Anything else is an error, so a typo cannot
// pass for "off".
func Parse(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" || v == Local {
		return v, nil
	}
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return "", fmt.Errorf(`CELLAR: want empty, %q or an http(s) URL without credentials`, Local)
	}
	return strings.TrimRight(v, "/"), nil
}
