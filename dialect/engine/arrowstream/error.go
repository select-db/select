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

// errorMetadata is the terminal metadata for err: its message, and its code
// when err or an error it wraps is an *Error with one.
func errorMetadata(err error) arrow.Metadata {
	var coded *Error
	if errors.As(err, &coded) && coded.Code != "" {
		return arrow.NewMetadata([]string{"error", "code"}, []string{err.Error(), coded.Code})
	}
	return arrow.NewMetadata([]string{"error"}, []string{err.Error()})
}

// errorFrom is the failure a segment's metadata carries, or nil.
func errorFrom(meta arrow.Metadata) *Error {
	idx := meta.FindKey("error")
	if idx < 0 {
		return nil
	}
	e := &Error{Message: meta.Values()[idx]}
	if idx := meta.FindKey("code"); idx >= 0 {
		e.Code = meta.Values()[idx]
	}
	return e
}
