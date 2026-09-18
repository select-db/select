package engine

import "sync"

// PageStatus indicates the completeness of a page returned by StreamingResult.Page.
type PageStatus int

const (
	// PageReady means the page contains rows[start:end] OR the stream finished
	// before reaching this page (caller should treat empty + done as end-of-data).
	PageReady PageStatus = iota
	// PagePartial means rows[start:available] are returned but more are coming
	// for this page. The caller should refetch when available advances.
	PagePartial
	// PagePending means no rows are available yet for this page.
	PagePending
)

// StreamingResult is the cached, append-during-streaming view of a query result.
// Writers use AppendRow / Finalize / Fail under the package's sink helpers.
// Readers use Page to retrieve a window of rows with completion status.
type StreamingResult struct {
	mu sync.RWMutex
	// id names the execution that owns this buffer. The cache is keyed by
	// (db, file), so a re-run replaces the entry under the same key; readers
	// carry the id they started with and compare, rather than trusting the
	// slot to still hold what they asked for.
	id             string
	columns        []string
	columnEditMeta []ColumnEditMeta
	rows           [][]any
	available      int64
	done           bool
	errMsg         string
	errPos         *int
	rowCount       int64
	affected       int64
	durationMs     int64
}

// NewStreamingResult returns an empty StreamingResult ready for appending,
// owned by the execution named by id.
func NewStreamingResult(id string) *StreamingResult {
	return &StreamingResult{id: id}
}

// ID returns the execution that owns this buffer.
func (s *StreamingResult) ID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.id
}

// SetColumns is called once when the query's column list is known.
// Subsequent calls are no-ops to keep schema stable for ongoing readers.
func (s *StreamingResult) SetColumns(cols []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.columns != nil {
		return
	}
	s.columns = cols
}

// SetColumnEditMeta attaches per-column editability metadata. Called once after
// the schema is known, before any consumer is likely to edit.
func (s *StreamingResult) SetColumnEditMeta(meta []ColumnEditMeta) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.columnEditMeta = meta
}

// SetExecutedDuration records the SQL execution duration as known when the
// engine finished executing (before all rows have streamed). Only used by
// the local path; the remote summary supplies durationMs at OnDone time.
func (s *StreamingResult) SetExecutedDuration(durationMs int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.durationMs == 0 {
		s.durationMs = durationMs
	}
}

// AppendRow writes a single row and bumps the available watermark.
func (s *StreamingResult) AppendRow(row []any) {
	s.mu.Lock()
	s.rows = append(s.rows, row)
	s.available = int64(len(s.rows))
	s.mu.Unlock()
}

// Finalize marks the stream as complete with a successful summary.
// Available stays bound to len(rows); the summary's rowCount is reported as-is
// and may differ for non-SELECT statements (where rowCount is 0 and affected
// carries the work).
func (s *StreamingResult) Finalize(rowCount, affected, durationMs int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.done = true
	s.rowCount = rowCount
	s.affected = affected
	s.durationMs = durationMs
}

// Fail marks the stream as terminated with an error. errPos is optional.
func (s *StreamingResult) Fail(msg string, errPos *int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.done = true
	s.errMsg = msg
	s.errPos = errPos
}

// Available returns the current row watermark (rows safe to read from index 0).
func (s *StreamingResult) Available() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.available
}

// Done reports whether the stream has terminated (either normally or with an error).
func (s *StreamingResult) Done() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.done
}

// Header returns the columns and edit metadata. Safe to call once SetColumns has fired.
func (s *StreamingResult) Header() (cols []string, meta []ColumnEditMeta) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.columns, s.columnEditMeta
}

// Summary returns the final summary fields. RowCount is 0 until Finalize is called.
func (s *StreamingResult) Summary() (rowCount, affected, durationMs int64, errMsg string, errPos *int, done bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rowCount, s.affected, s.durationMs, s.errMsg, s.errPos, s.done
}

// PageData is the row window returned by Page. Always carries the columns and
// summary fields known at read time; rows is nil when no data is available yet.
type PageData struct {
	// ID is the execution this page came from. Callers that hold an execution
	// id must check it: the cache slot may have been taken over since.
	ID             string
	Columns        []string
	Rows           [][]any
	ColumnEditMeta []ColumnEditMeta
	RowCount       int64
	AffectedRows   int64
	DurationMs     int64
	Available      int64
	Errors         []string
	ErrorPosition  *int
	Page           int
	PageSize       int
}

// Page returns rows[start:end] from the buffer, capped to the available watermark.
// Status indicates whether the returned page is complete, partial (more coming
// for this window), or pending (no rows yet for this window).
func (s *StreamingResult) Page(page, pageSize int) (*PageData, PageStatus) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	start := page * pageSize
	end := start + pageSize
	available := int(s.available)

	header := &PageData{
		ID:             s.id,
		Columns:        s.columns,
		ColumnEditMeta: s.columnEditMeta,
		RowCount:       s.rowCount,
		AffectedRows:   s.affected,
		DurationMs:     s.durationMs,
		Available:      s.available,
		ErrorPosition:  s.errPos,
		Page:           page,
		PageSize:       pageSize,
	}
	if s.errMsg != "" {
		header.Errors = []string{s.errMsg}
	}

	if start >= available {
		if s.done {
			// Page lies past the end of a finished stream; return the empty page
			// with PageReady so callers stop requesting beyond the result.
			return header, PageReady
		}
		return header, PagePending
	}

	realEnd := end
	if realEnd > available {
		realEnd = available
	}

	rows := make([][]any, realEnd-start)
	copy(rows, s.rows[start:realEnd])
	header.Rows = rows

	status := PageReady
	if realEnd < end && !s.done {
		status = PagePartial
	}
	return header, status
}
