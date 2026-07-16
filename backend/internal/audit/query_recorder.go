package audit

import (
	"context"
	"errors"
	"sync"

	core "github.com/selectDb/dialect/core"
)

// QueryRecorder finalizes and emits the single query.executed event for one query
// run through an engine RowSink. It owns the outcome rules — a permission block is
// StatusDenied (valid identity, missing grant), any other error is StatusError —
// and the emit-once guard, so every sink that wraps the engine (the datasource
// proxy, the MCP tools) records queries identically instead of re-implementing it.
//
// A nil recorder, or one built from a nil Record, is a no-op: internal queries
// that read no data (EXPLAIN/plan) opt out of the data-plane log.
type QueryRecorder struct {
	rec  *Record
	once sync.Once
}

func NewQueryRecorder(rec *Record) *QueryRecorder {
	return &QueryRecorder{rec: rec}
}

// Executed records the execute-phase duration, before rows stream. Optional: a
// sink that only sees a total duration can skip it and pass it to Success.
func (q *QueryRecorder) Executed(durationMs int64) {
	if q == nil || q.rec == nil {
		return
	}
	q.rec.DurationMs = durationMs
}

// Success finalizes and emits a successful run. A duration already set by
// Executed wins over the total passed here.
func (q *QueryRecorder) Success(rowCount, affected, durationMs int64) {
	q.finish(func() {
		q.rec.ReturnedRowCount = rowCount
		if affected > 0 {
			q.rec.Payload["affected_rows"] = affected
		}
		if q.rec.DurationMs == 0 {
			q.rec.DurationMs = durationMs
		}
		q.rec.Status = StatusSuccess
	})
}

// Failure finalizes and emits a failed run, classifying a permission block as a
// denied outcome rather than a system fault.
func (q *QueryRecorder) Failure(err error) {
	q.finish(func() {
		q.rec.Status = statusForQueryError(err)
		q.rec.Payload["error_message"] = err.Error()
	})
}

func (q *QueryRecorder) finish(set func()) {
	if q == nil || q.rec == nil {
		return
	}
	// A run ends in Success or Failure, never both, but the guard makes that
	// explicit. Background context so the async lane isn't cancelled with the request.
	q.once.Do(func() {
		set()
		_ = Emit(context.Background(), QueryExecuted, *q.rec)
	})
}

// statusForQueryError maps the engine's error to a status: the permission engine
// rejects a blocked query before it runs (engine.StreamLocal) with a typed
// PermissionDeniedError; everything else is a genuine fault.
func statusForQueryError(err error) string {
	var denied *core.PermissionDeniedError
	if errors.As(err, &denied) {
		return StatusDenied
	}
	return StatusError
}
