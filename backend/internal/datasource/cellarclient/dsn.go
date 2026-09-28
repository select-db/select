package cellarclient

import (
	"net/url"
	"strconv"

	"backend/internal/cellar"

	"github.com/selectDb/dialect/engine/arrowstream"
	"github.com/selectDb/dialect/sqlite"
)

// ErrOff answers any use of a managed database while CELLAR is unset.
var ErrOff error = &arrowstream.Error{Code: cellar.CodeDisabled, Message: "managed databases are not enabled on this server"}

// Plan is what a workspace plan allows its managed databases.
type Plan struct {
	DatabaseMaxBytes  int64
	WorkspaceMaxBytes int64 // all its managed databases together
	MaxDatabases      int64
	PointInTimeDays   int // how far back a fork may reach
}

// Plans by workspace.plan. An unknown plan has no size cap, which the cellar
// refuses, and room for no database.
var Plans = map[string]Plan{
	"solo":  {DatabaseMaxBytes: 250 << 20, WorkspaceMaxBytes: 1 << 30, MaxDatabases: 10, PointInTimeDays: 1},
	"teams": {DatabaseMaxBytes: 1 << 30, WorkspaceMaxBytes: 20 << 30, MaxDatabases: 100, PointInTimeDays: 7},
}

// URL is the cellar managed databases run on, and CellarID the name new ones
// record for it; both set once at startup. URL "" means managed databases are off.
var URL, CellarID string

// DSN is the DSN of datasource id, kept on cellar cellarID, with its
// workspace's limits: the plan's size cap and two statements per member.
func DSN(cellarID, id, workspaceID, plan string, members int) string {
	params := url.Values{
		"workspace_id":  {workspaceID},
		"max_bytes":     {strconv.FormatInt(Plans[plan].DatabaseMaxBytes, 10)},
		"max_in_flight": {strconv.Itoa(max(4, 2*members))},
	}
	return (&url.URL{Scheme: sqlite.CellarDriver, Host: cellarID, Path: "/" + id, RawQuery: params.Encode()}).String()
}
