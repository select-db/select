package datasource

import (
	"context"
	"net/http"
	"sync"

	"backend/internal/audit"
	"backend/internal/auth"
	"backend/internal/authz"
	"backend/internal/middlewares"

	"github.com/selectDb/dialect/engine/arrowstream"
)

// loggingSink captures a query's outcome and emits it. It embeds *arrowstream.Sink
// so the sink's methods (incl. those the engine type-asserts) are promoted; only
// the terminal ones are overridden, and the event is emitted once.
type loggingSink struct {
	*arrowstream.Sink

	rec  audit.Record
	once sync.Once
}

func newLoggingSink(inner *arrowstream.Sink, rec audit.Record) *loggingSink {
	return &loggingSink{Sink: inner, rec: rec}
}

func (s *loggingSink) OnExecuted(durationMs int64) {
	s.rec.DurationMs = durationMs
	s.Sink.OnExecuted(durationMs)
}

func (s *loggingSink) OnDone(rowCount, affected, durationMs int64) error {
	err := s.Sink.OnDone(rowCount, affected, durationMs)
	s.rec.ReturnedRowCount = rowCount
	if affected > 0 {
		s.rec.Payload["affected_rows"] = affected
	}
	if s.rec.DurationMs == 0 {
		s.rec.DurationMs = durationMs
	}
	s.rec.Status = audit.StatusSuccess
	s.emit()
	return err
}

func (s *loggingSink) OnError(err error) {
	s.Sink.OnError(err)
	s.rec.Status = audit.StatusError
	s.rec.Payload["error_message"] = err.Error()
	s.emit()
}

func (s *loggingSink) emit() {
	s.once.Do(func() { _ = audit.Emit(context.Background(), audit.QueryExecuted, s.rec) })
}

func buildQueryRecord(r *http.Request, req executeRequest, dbType string) audit.Record {
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
