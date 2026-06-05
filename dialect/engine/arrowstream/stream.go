package arrowstream

import (
	"fmt"
	"io"
	"strconv"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/klauspost/compress/zstd"
)

// Stream decodes Arrow IPC + zstd from a reader.
// Implements the read side: Columns → Next* → Summary → Close.
type Stream struct {
	body    io.ReadCloser
	zr      *zstd.Decoder
	reader  *ipc.Reader
	cols    []string
	row     int
	numRows int
	done    bool

	// executedMs carries the engine's QueryContext duration when the sender
	// emits an early "executed" stream segment ahead of the data stream.
	executedMs int64

	// summary fields populated after last record
	rowCount   int64
	affected   int64
	durationMs int64
	streamErr  string
}

// NewStream wraps a zstd-compressed Arrow IPC response body.
func NewStream(body io.ReadCloser) (*Stream, error) {
	zr, err := zstd.NewReader(body)
	if err != nil {
		_ = body.Close()
		return nil, fmt.Errorf("zstd reader: %w", err)
	}
	return &Stream{body: body, zr: zr}, nil
}

// Columns reads the first record batch and returns column names. If the
// sender prepended an "executed" segment, its durationMs is captured and
// available via Executed() before any rows are consumed.
func (s *Stream) Columns() ([]string, error) {
	reader, err := ipc.NewReader(s.zr)
	if err != nil {
		return nil, fmt.Errorf("arrow reader: %w", err)
	}

	meta := reader.Schema().Metadata()

	// Check for immediate error (query failed before any rows)
	if idx := meta.FindKey("error"); idx >= 0 {
		s.streamErr = meta.Values()[idx]
		s.done = true
		reader.Release()
		return nil, fmt.Errorf("%s", s.streamErr)
	}

	// Optional early-duration segment: a header-only stream whose schema
	// metadata carries `executed_ms`. Drain the end-of-stream marker before
	// opening the next IPC stream as the actual data segment.
	if idx := meta.FindKey("executed_ms"); idx >= 0 {
		s.executedMs, _ = strconv.ParseInt(meta.Values()[idx], 10, 64)
		for reader.Next() {
			// drain any (empty) record batches up to the end-of-stream marker
		}
		reader.Release()
		reader, err = ipc.NewReader(s.zr)
		if err != nil {
			return nil, fmt.Errorf("arrow reader: %w", err)
		}
		meta = reader.Schema().Metadata()
		if idx := meta.FindKey("error"); idx >= 0 {
			s.streamErr = meta.Values()[idx]
			s.done = true
			reader.Release()
			return nil, fmt.Errorf("%s", s.streamErr)
		}
	}

	s.reader = reader
	schema := reader.Schema()
	s.cols = make([]string, schema.NumFields())
	for i, f := range schema.Fields() {
		s.cols[i] = f.Name
	}

	return s.cols, nil
}

// Executed returns the SQL execution duration captured from the optional
// "executed" segment, or 0 if the sender didn't emit one. Valid after Columns.
func (s *Stream) Executed() int64 {
	return s.executedMs
}

// Next returns the next row as []any (all strings or nil).
// Returns nil, false, nil when exhausted.
func (s *Stream) Next() ([]any, bool, error) {
	for {
		// Current record batch has rows remaining
		if s.reader != nil && s.row < s.numRows {
			rec := s.reader.RecordBatch()
			row := make([]any, len(s.cols))
			for i := range s.cols {
				col := rec.Column(i)
				if col.IsNull(s.row) {
					row[i] = nil
				} else {
					row[i] = col.ValueStr(s.row)
				}
			}
			s.row++
			return row, true, nil
		}

		// Try to read next record in current stream
		if s.reader != nil && s.reader.Next() {
			rec := s.reader.RecordBatch()
			s.row = 0
			s.numRows = int(rec.NumRows())

			// Check for terminal metadata (done or error)
			if meta := s.reader.Schema().Metadata(); meta.Len() > 0 {
				if idx := meta.FindKey("done"); idx >= 0 {
					s.parseSummary(meta)
					s.done = true
					return nil, false, nil
				}
				if idx := meta.FindKey("error"); idx >= 0 {
					s.streamErr = meta.Values()[idx]
					s.done = true
					return nil, false, fmt.Errorf("%s", s.streamErr)
				}
			}

			if s.numRows > 0 {
				continue
			}
		}

		// Current reader exhausted, try opening next stream (summary/error stream follows data)
		if s.reader != nil {
			s.reader.Release()
			s.reader = nil
		}

		reader, err := ipc.NewReader(s.zr)
		if err != nil {
			// EOF = no more streams, we're done
			s.done = true
			return nil, false, nil
		}
		s.reader = reader
		s.row = 0
		s.numRows = 0

		// Check schema metadata of new stream
		if meta := reader.Schema().Metadata(); meta.Len() > 0 {
			if idx := meta.FindKey("done"); idx >= 0 {
				s.parseSummary(meta)
				s.done = true
				return nil, false, nil
			}
			if idx := meta.FindKey("error"); idx >= 0 {
				s.streamErr = meta.Values()[idx]
				s.done = true
				return nil, false, fmt.Errorf("%s", s.streamErr)
			}
		}
	}
}

// Summary returns query summary. Only valid after Next returns false.
func (s *Stream) Summary() (rowCount, affected, durationMs int64, err error) {
	if s.streamErr != "" {
		return 0, 0, 0, fmt.Errorf("%s", s.streamErr)
	}
	return s.rowCount, s.affected, s.durationMs, nil
}

// Close releases all resources.
func (s *Stream) Close() error {
	if s.reader != nil {
		s.reader.Release()
	}
	if s.zr != nil {
		s.zr.Close()
	}
	return s.body.Close()
}

func (s *Stream) parseSummary(meta arrow.Metadata) {
	if idx := meta.FindKey("rows"); idx >= 0 {
		s.rowCount, _ = strconv.ParseInt(meta.Values()[idx], 10, 64)
	}
	if idx := meta.FindKey("affected"); idx >= 0 {
		s.affected, _ = strconv.ParseInt(meta.Values()[idx], 10, 64)
	}
	if idx := meta.FindKey("ms"); idx >= 0 {
		s.durationMs, _ = strconv.ParseInt(meta.Values()[idx], 10, 64)
	}
}
