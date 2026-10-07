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

// URL is the cellar managed databases run on, set once at startup. "" means managed
// databases are off.
var URL string

// DSN is the DSN of datasource id, with its workspace's limits: a size cap and two
// statements per member.
func DSN(id, workspaceID string, maxBytes int64, members int) string {
	params := url.Values{
		"workspace_id":  {workspaceID},
		"max_bytes":     {strconv.FormatInt(maxBytes, 10)},
		"max_in_flight": {strconv.Itoa(max(4, 2*members))},
	}
	return (&url.URL{Scheme: sqlite.CellarDriver, Path: "/" + id, RawQuery: params.Encode()}).String()
}
