package cellar

import (
	"errors"
	"net/url"
	"strconv"
)

// ErrOff answers any use of a managed database while CELLAR is unset.
var ErrOff = errors.New("managed databases are not enabled on this server")

// maxBytes is what a managed database's workspace plan lets it grow to. An
// unknown plan has no size cap, which the cellar refuses.
var maxBytes = map[string]int64{
	"solo":  250 << 20,
	"teams": 1 << 30,
}

// base is the URL of the cellar managed databases run on; "" while off.
var base string

// Use sends managed databases to the cellar at url. Call once at startup.
func Use(url string) { base = url }

// DSN is the DSN of datasource id, kept on cellar cellarID, with its
// workspace's limits: the plan's size cap and two statements per member.
func DSN(cellarID, id, workspaceID, plan string, members int) string {
	q := url.Values{
		"workspace_id":  {workspaceID},
		"max_bytes":     {strconv.FormatInt(maxBytes[plan], 10)},
		"max_in_flight": {strconv.Itoa(max(4, 2*members))},
	}
	return (&url.URL{Scheme: Scheme, Host: cellarID, Path: "/" + id, RawQuery: q.Encode()}).String()
}
