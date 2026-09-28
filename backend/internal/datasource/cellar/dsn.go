package cellar

import (
	"net/url"
	"strconv"

	server "backend/internal/cellar"

	"github.com/selectDb/dialect/engine/arrowstream"
)

// ErrOff answers any use of a managed database while CELLAR is unset.
var ErrOff error = &arrowstream.Error{Code: server.CodeDisabled, Message: "managed databases are not enabled on this server"}

// Plan is what a workspace plan allows its managed databases.
type Plan struct {
	MaxBytes   int64 // one database
	TotalBytes int64 // all of the workspace's
	MaxDBs     int64
	PITRDays   int // how far back a fork may reach
}

// Plans by workspace.plan. An unknown plan has no size cap, which the cellar
// refuses, and room for no database.
var Plans = map[string]Plan{
	"solo":  {MaxBytes: 250 << 20, TotalBytes: 1 << 30, MaxDBs: 10, PITRDays: 1},
	"teams": {MaxBytes: 1 << 30, TotalBytes: 20 << 30, MaxDBs: 100, PITRDays: 7},
}

// URL is the cellar managed databases run on, and ID the name new ones record
// for it; both set once at startup. URL "" means managed databases are off.
var URL, ID string

// DSN is the DSN of datasource id, kept on cellar cellarID, with its
// workspace's limits: the plan's size cap and two statements per member.
func DSN(cellarID, id, workspaceID, plan string, members int) string {
	q := url.Values{
		"workspace_id":  {workspaceID},
		"max_bytes":     {strconv.FormatInt(Plans[plan].MaxBytes, 10)},
		"max_in_flight": {strconv.Itoa(max(4, 2*members))},
	}
	return (&url.URL{Scheme: Scheme, Host: cellarID, Path: "/" + id, RawQuery: q.Encode()}).String()
}
