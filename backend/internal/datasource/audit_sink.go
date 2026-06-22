package datasource

import (
	"context"
	"net/http"
	"sync"

	"backend/internal/audit"
	"backend/internal/auth"
	"backend/internal/authz"

	"github.com/selectDb/dialect/engine/arrowstream"
)

// loggingSink wraps the Arrow sink to capture a query's outcome and emit it via
// the audit catalog. It embeds *arrowstream.Sink so all of the sink's concrete
// methods are promoted unchanged; only the terminal methods are overridden to
// record the result, and the event is emitted exactly once.
type loggingSink struct {
	*arrowstream.Sink

	rec  audit.Record
	once sync.Once
}

func newLoggingSink(inner *arrowstream.Sink, rec audit.Record) *loggingSink {
	return &loggingSink{Sink: inner, rec: rec}
}

// OnExecuted fires once the statement has run but before rows stream; it carries
// the truest execution latency.
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

// buildQueryRecord assembles the per-event data for a single execute request.
// The principal (identity + roles + permissions, as-of-now) is built by the
// shared authz.RequestPrincipal; the SQL is recorded in the plaintext payload
// (Tier 0 encryption-at-rest). The envelope (domain, action, target type, lane)
// comes from audit.QueryExecuted.
func buildQueryRecord(r *http.Request, req executeRequest, dbType string) audit.Record {
	p := authz.RequestPrincipal(r)
	return audit.Record{
		WorkspaceID: p.WorkspaceID,
		Principal:   p,
		TargetID:    req.ID,
		Payload: map[string]any{
			"sql_text": req.SQL,
			"db_type":  dbType,
		},
		ClientIP: auth.GetIPAddress(r),
	}
}
