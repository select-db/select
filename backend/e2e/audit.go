package e2e

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"backend/internal/audit"
)

// pollTimeout bounds how long RequireEvent waits for the async/outbox lanes to
// land a row; the harness runs both lanes with short intervals.
const pollTimeout = 3 * time.Second

// StartAudit builds an audit logger bound to conn, runs its lanes with test-fast
// intervals, installs it as the process default, and stops it on cleanup. Must be
// called after NewDB so the logger and the emit sites share the same database.
func StartAudit(t *testing.T, conn *sql.DB) *audit.Logger {
	t.Helper()
	logger := audit.New(conn, audit.Options{
		FlushInterval: 50 * time.Millisecond,
		OutboxPoll:    50 * time.Millisecond,
	})
	logger.Start()
	audit.SetDefault(logger)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = logger.Stop(ctx)
		audit.SetDefault(nil)
	})
	return logger
}

// CountEvents returns how many audit.event rows match domain.action (status
// filtered too when non-empty).
func CountEvents(t *testing.T, conn *sql.DB, domain, action, status string) int {
	t.Helper()
	q := `SELECT count(*) FROM audit.event WHERE domain = $1 AND action = $2`
	args := []any{domain, action}
	if status != "" {
		q += ` AND status = $3`
		args = append(args, status)
	}
	var n int
	require.NoError(t, conn.QueryRow(q, args...).Scan(&n))
	return n
}

// RequireEvent asserts that at least one domain.action event is persisted,
// polling until the lanes flush. Use for wired specs; an unwired spec fails here
// (the intended red).
func RequireEvent(t *testing.T, conn *sql.DB, domain, action string) {
	t.Helper()
	require.Eventuallyf(t, func() bool {
		return CountEvents(t, conn, domain, action, "") > 0
	}, pollTimeout, 25*time.Millisecond, "expected audit event %s.%s to be persisted", domain, action)
}

// RequireEventStatus is RequireEvent scoped to a status (e.g. denied, error).
func RequireEventStatus(t *testing.T, conn *sql.DB, domain, action, status string) {
	t.Helper()
	require.Eventuallyf(t, func() bool {
		return CountEvents(t, conn, domain, action, status) > 0
	}, pollTimeout, 25*time.Millisecond, "expected audit event %s.%s status=%s to be persisted", domain, action, status)
}
