package cellar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"backend/internal/utils"

	"github.com/selectDb/dialect/engine/arrowstream"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// The codes a managed database's failure carries to the backend and on to the
// caller. Anything the cellar cannot place is CodeInternal.
const (
	CodeSQLError           = "sql_error"
	CodeForbiddenStatement = "forbidden_statement"
	CodeQuotaExceeded      = "quota_exceeded"
	CodeTimeout            = "timeout"
	CodeWaking             = "waking"
	CodeUnavailable        = "unavailable"
	CodeDisabled           = "disabled"
	CodeInternal           = "internal"
)

// classify gives err its code and the message the caller may see:
//   - SQLite's own message only for errors about the statement, never for
//     file, I/O or corruption errors, whose text can name a path
//   - an internal error only as a ref; its detail goes to the log
func classify(ctx context.Context, err error, grant Grant) *arrowstream.Error {
	var coded *arrowstream.Error
	var sqliteErr *sqlite.Error
	switch {
	case errors.As(err, &coded):
		return coded
	case errors.Is(err, ErrForbiddenStatement):
		return &arrowstream.Error{Code: CodeForbiddenStatement, Message: err.Error()}
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return &arrowstream.Error{Code: CodeTimeout, Message: fmt.Sprintf("query exceeded the %.0fs limit on managed databases", statementTimeout.Seconds())}
	case errors.As(err, &sqliteErr):
		switch sqliteErr.Code() & 0xff {
		case sqlite3.SQLITE_ERROR, sqlite3.SQLITE_CONSTRAINT, sqlite3.SQLITE_MISMATCH, sqlite3.SQLITE_RANGE, sqlite3.SQLITE_TOOBIG:
			return &arrowstream.Error{Code: CodeSQLError, Message: err.Error()}
		case sqlite3.SQLITE_FULL:
			return &arrowstream.Error{Code: CodeQuotaExceeded, Message: fmt.Sprintf("database is over its %d MB limit", grant.MaxBytes>>20)}
		case sqlite3.SQLITE_BUSY, sqlite3.SQLITE_LOCKED:
			return &arrowstream.Error{Code: CodeUnavailable, Message: "database busy, retry"}
		}
	}
	return InternalError(fmt.Sprintf("cellar: datasource %s: %v", grant.DatasourceID, err))
}

// InternalError logs detail under a ref and returns the error the caller sees,
// which names only the ref.
func InternalError(detail string) *arrowstream.Error {
	ref := utils.LogWithRef(detail)
	return &arrowstream.Error{Code: CodeInternal, Message: "internal error, ref " + ref}
}

// classifiedSink classifies every failure before it is written to the stream.
type classifiedSink struct {
	*arrowstream.Sink
	ctx   context.Context
	grant Grant
}

func (s classifiedSink) OnError(err error) {
	s.Sink.OnError(classify(s.ctx, err, s.grant))
}

var (
	errAlreadyExists = &arrowstream.Error{Code: CodeSQLError, Message: "a managed database with this id already exists"}
	errNotFound      = &arrowstream.Error{Code: CodeSQLError, Message: "managed database not found"}
	errNoCopyAtTime  = &arrowstream.Error{Code: CodeSQLError, Message: "no copy of the managed database exists at that time"}
	errWaking        = &arrowstream.Error{Code: CodeWaking, Message: "managed database is waking up, retry in a few seconds"}
)

// writeLifecycleError answers with the failure's code and message as JSON.
func writeLifecycleError(w http.ResponseWriter, r *http.Request, err error) {
	coded := classify(r.Context(), err, GetGrant(r))
	status := http.StatusBadRequest
	switch {
	case coded == errAlreadyExists:
		status = http.StatusConflict
	case coded == errNotFound:
		status = http.StatusNotFound
	case coded.Code == CodeWaking:
		status = http.StatusServiceUnavailable
		w.Header().Set("Retry-After", "5")
	case coded.Code == CodeInternal:
		status = http.StatusInternalServerError
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(coded)
}
