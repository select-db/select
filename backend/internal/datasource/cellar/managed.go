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

// URL is the cellar managed databases run on, set once at startup; "" means
// they are off.
var URL string

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
