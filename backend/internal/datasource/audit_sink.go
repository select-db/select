package datasource

import (
	"net/http"

	"backend/internal/audit"
	"backend/internal/auth"
	"backend/internal/authz"
	"backend/internal/middlewares"

	"github.com/selectDb/dialect/engine/arrowstream"
)

// loggingSink captures a query's outcome and emits it. It embeds *arrowstream.Sink
// so the sink's methods (incl. those the engine type-asserts) are promoted; only
// the terminal ones are overridden. The audit rules (status, emit-once, denied
// classification) live in audit.QueryRecorder, shared with the MCP query sink.
type loggingSink struct {
	*arrowstream.Sink

	audit *audit.QueryRecorder
}

func newLoggingSink(inner *arrowstream.Sink, rec audit.Record) *loggingSink {
	return &loggingSink{Sink: inner, audit: audit.NewQueryRecorder(&rec)}
}

func (s *loggingSink) OnExecuted(durationMs int64) {
	s.audit.Executed(durationMs)
	s.Sink.OnExecuted(durationMs)
}

func (s *loggingSink) OnDone(rowCount, affected, durationMs int64) error {
	err := s.Sink.OnDone(rowCount, affected, durationMs)
	s.audit.Success(rowCount, affected, durationMs)
	return err
}

func (s *loggingSink) OnError(err error) {
	s.Sink.OnError(err)
	s.audit.Failure(err)
}

func newQueryAuditRecord(r *http.Request, req executeRequest, dbType string) audit.Record {
	// Execute runs behind Membership(), so the member workspace is set.
	p := authz.RequestPrincipal(r, middlewares.MemberWorkspaceID(r))
	return audit.Record{
		WorkspaceID: p.WorkspaceID,
		Principal:   p,
		TargetID:    req.ID,
		Payload: map[string]any{
			"sql_text": req.SQL,
			"db_type":  dbType,
			"channel":  "app",
		},
		ClientIP: auth.GetIPAddress(r),
	}
}
