package arrowstream

import (
	"errors"

	"github.com/apache/arrow-go/v18/arrow"
)

// Error is a failure a sender wrote into the stream, with its code when the
// sender gave one.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

// ErrorCode lets a reader of any wrapped error find the code.
func (e *Error) ErrorCode() string {
	return e.Code
}

// errorMetadata is the terminal metadata for err: its message, and its code
// when err or an error it wraps names one.
func errorMetadata(err error) arrow.Metadata {
	var coded interface{ ErrorCode() string }
	if errors.As(err, &coded) && coded.ErrorCode() != "" {
		return arrow.NewMetadata([]string{"error", "code"}, []string{err.Error(), coded.ErrorCode()})
	}
	return arrow.NewMetadata([]string{"error"}, []string{err.Error()})
}

func errorFrom(meta arrow.Metadata) *Error {
	e := &Error{Message: meta.Values()[meta.FindKey("error")]}
	if idx := meta.FindKey("code"); idx >= 0 {
		e.Code = meta.Values()[idx]
	}
	return e
}
