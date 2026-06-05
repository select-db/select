package engine

import (
	"sync"
	"time"
)

// StreamListener observes lifecycle events on a streaming query.
// All callbacks may be invoked from a goroutine other than the one that
// created the stream. Implementations must be safe for concurrent use only
// in the sense that callbacks are serialised by the sink itself.
type StreamListener interface {
	// OnStart fires once, before any OnProgress, when columns are known.
	OnStart(columns []string, columnEditMeta []ColumnEditMeta)
	// OnExecuted fires once when the engine has finished SQL execution and is
	// about to start streaming rows. Only the local path emits this; for
	// proxified queries the durationMs is only known via the final summary.
	OnExecuted(durationMs int64)
	// OnProgress reports the latest row watermark. Throttled by the sink.
	OnProgress(available int64)
	// OnDone fires once when the stream finishes successfully.
	OnDone(rowCount, affected, durationMs int64)
	// OnError fires once when the stream terminates abnormally.
	OnError(message string, errorPosition *int)
}

// streamingSink is a RowSink that writes rows into a StreamingResult and
// notifies a StreamListener on a fixed cadence.
type streamingSink struct {
	result   *StreamingResult
	listener StreamListener

	progressEvery time.Duration
	progressBatch int

	mu              sync.Mutex
	rowsSinceFlush  int
	lastFlush       time.Time
	startedNotified bool
}

const (
	defaultProgressInterval = 40 * time.Millisecond
	defaultProgressBatch    = 200
)

// NewStreamingSink wires a RowSink to a StreamingResult + listener.
// Pass nil for listener if no notification is needed (the result still fills).
func NewStreamingSink(result *StreamingResult, listener StreamListener) RowSink {
	return &streamingSink{
		result:        result,
		listener:      listener,
		progressEvery: defaultProgressInterval,
		progressBatch: defaultProgressBatch,
		lastFlush:     time.Now(),
	}
}

func (s *streamingSink) OnColumns(cols []string) error {
	s.result.SetColumns(cols)
	if s.listener != nil && !s.startedNotified {
		s.startedNotified = true
		columns, meta := s.result.Header()
		s.listener.OnStart(columns, meta)
	}
	return nil
}

// OnExecuted is the optional hook engines can call to surface the SQL
// execution duration before all rows have streamed. Local execution emits
// this right after the driver returns from QueryContext.
func (s *streamingSink) OnExecuted(durationMs int64) {
	s.result.SetExecutedDuration(durationMs)
	if s.listener != nil {
		s.listener.OnExecuted(durationMs)
	}
}

func (s *streamingSink) OnRow(values []any) error {
	s.result.AppendRow(values)

	s.mu.Lock()
	s.rowsSinceFlush++
	shouldFlush := s.rowsSinceFlush >= s.progressBatch ||
		time.Since(s.lastFlush) >= s.progressEvery
	if shouldFlush {
		s.rowsSinceFlush = 0
		s.lastFlush = time.Now()
	}
	s.mu.Unlock()

	if shouldFlush && s.listener != nil {
		s.listener.OnProgress(s.result.Available())
	}
	return nil
}

func (s *streamingSink) OnDone(rowCount, affected, durationMs int64) error {
	s.result.Finalize(rowCount, affected, durationMs)
	if s.listener != nil {
		s.listener.OnDone(rowCount, affected, durationMs)
	}
	return nil
}

func (s *streamingSink) OnError(err error) {
	if err == nil {
		return
	}
	s.result.Fail(err.Error(), nil)
	if s.listener != nil {
		s.listener.OnError(err.Error(), nil)
	}
}

// drainStreamInto pulls rows from a remote RowStream into a StreamingResult
// via the supplied listener. Used by the proxified path.
func drainStreamInto(stream RowStream, sink RowSink) {
	defer func() { _ = stream.Close() }()

	cols, err := stream.Columns()
	if err != nil {
		sink.OnError(err)
		return
	}
	if err := sink.OnColumns(cols); err != nil {
		sink.OnError(err)
		return
	}

	// Forward the early SQL-execution duration when the wire format carries
	// it. Streams that don't expose it return 0; sinks that don't care
	// implement nothing.
	if e, ok := stream.(interface{ Executed() int64 }); ok {
		if executed := e.Executed(); executed > 0 {
			if n, ok := sink.(interface{ OnExecuted(int64) }); ok {
				n.OnExecuted(executed)
			}
		}
	}

	for {
		values, ok, err := stream.Next()
		if err != nil {
			sink.OnError(err)
			return
		}
		if !ok {
			break
		}
		if err := sink.OnRow(values); err != nil {
			sink.OnError(err)
			return
		}
	}

	rowCount, affected, durationMs, err := stream.Summary()
	if err != nil {
		sink.OnError(err)
		return
	}
	_ = sink.OnDone(rowCount, affected, durationMs)
}
