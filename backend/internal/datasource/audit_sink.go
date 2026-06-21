package datasource

import (
	"net/http"
	"sync"

	"backend/internal/audit"
	"backend/internal/auth"
	"backend/internal/authz"
	"backend/internal/middlewares"

	"github.com/selectDb/dialect/engine/arrowstream"
)

// loggingSink wraps the Arrow sink to capture a query's outcome for the audit
// log. It embeds *arrowstream.Sink so all of the sink's concrete methods
// (SetColumnTypes, SetDownstreamFlusher, OnColumns, OnRow, Close, ...) are
// promoted unchanged — including the optional interfaces the engine
// type-asserts. Only the terminal methods are overridden to record the result.
type loggingSink struct {
	*arrowstream.Sink

	ev   *audit.Event
	once sync.Once
}

func newLoggingSink(inner *arrowstream.Sink, ev *audit.Event) *loggingSink {
	return &loggingSink{Sink: inner, ev: ev}
}

// OnExecuted fires once the statement has run but before rows stream; it carries
// the truest execution latency.
func (s *loggingSink) OnExecuted(durationMs int64) {
	s.ev.DurationMs = durationMs
	s.Sink.OnExecuted(durationMs)
}

func (s *loggingSink) OnDone(rowCount, affected, durationMs int64) error {
	err := s.Sink.OnDone(rowCount, affected, durationMs)
	s.ev.ReturnedRowCount = rowCount
	if affected > 0 {
		s.ev.Payload["affected_rows"] = affected
	}
	if s.ev.DurationMs == 0 {
		s.ev.DurationMs = durationMs
	}
	s.ev.Status = audit.StatusSuccess
	s.emit()
	return err
}

func (s *loggingSink) OnError(err error) {
	s.Sink.OnError(err)
	s.ev.Status = audit.StatusError
	s.ev.Payload["error_message"] = err.Error()
	s.emit()
}

func (s *loggingSink) emit() {
	s.once.Do(func() { audit.Log(s.ev) })
}

// buildQueryEvent assembles the audit event for a single execute request. It
// snapshots the principal's identity + authz state as-of-now (cheap: roles and
// permissions are already resolved/cached in the request) and records the SQL
// in the plaintext payload (Tier 0 encryption-at-rest).
func buildQueryEvent(r *http.Request, req executeRequest, dbType string) *audit.Event {
	workspaceID := middlewares.MemberWorkspaceID(r)

	principalType := audit.PrincipalUser
	if middlewares.IsAPIKeyPrincipal(r) {
		principalType = audit.PrincipalAPIKey
	}

	return &audit.Event{
		WorkspaceID: workspaceID,
		Domain:      audit.DomainQuery,
		Action:      audit.ActionExecuted,
		Principal: audit.Principal{
			Type:        principalType,
			ID:          middlewares.GetUserID(r),
			Name:        middlewares.GetPrincipalName(r),
			WorkspaceID: workspaceID,
			Roles:       toAuditRoles(middlewares.GetRoles(r)),
			Permissions: authz.EntriesFromRequest(r),
		},
		Target: &audit.Target{
			Type: "datasource",
			ID:   req.ID,
		},
		Status: audit.StatusSuccess,
		Payload: map[string]any{
			"sql_text": req.SQL,
			"db_type":  dbType,
		},
		ClientIP: auth.GetIPAddress(r),
	}
}

// toAuditRoles maps the middleware's role refs into the audit snapshot type.
func toAuditRoles(refs []auth.RoleRef) []audit.Role {
	roles := make([]audit.Role, len(refs))
	for i, r := range refs {
		roles[i] = audit.Role{ID: r.ID, Name: r.Name}
	}
	return roles
}
