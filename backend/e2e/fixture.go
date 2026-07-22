package e2e

import (
	"database/sql"
	"net/http"
	"testing"
)

// Fixture is the standard per-test environment: an isolated migrated database, a
// crafted owner identity with a minted token, and the real application handler.
type Fixture struct {
	Conn  *sql.DB
	Actor Actor
	H     http.Handler
}

// Setup builds a Fixture: a fresh isolated DB, a running audit logger, a crafted
// owner account, and the handler — the four things every e2e test needs, in one
// call. Cleanup is registered via t.Cleanup by the underlying helpers.
func Setup(t *testing.T) Fixture {
	t.Helper()
	conn := NewDB(t)
	StartAudit(t, conn)
	return Fixture{
		Conn:  conn,
		Actor: NewAccount(t, conn),
		H:     NewHandler(),
	}
}
