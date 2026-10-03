package managed

import (
	"net/http"

	"backend/internal/cellar"
)

// Refusal is a request the caller can correct, answered with its HTTP status.
type Refusal struct {
	Status  int
	Message string
}

func (refusal *Refusal) Error() string { return refusal.Message }

var (
	errForbidden = &Refusal{http.StatusForbidden, "forbidden"}
	// ErrNotFound is a managed database the workspace does not have, or no longer serves.
	ErrNotFound = &Refusal{http.StatusNotFound, "datasource not found"}
)

// CodeStatus is the HTTP status of each code a managed database's failure carries.
var CodeStatus = map[string]int{
	cellar.CodeSQLError:           http.StatusBadRequest,
	cellar.CodeForbiddenStatement: http.StatusBadRequest,
	cellar.CodeQuotaExceeded:      http.StatusForbidden,
	cellar.CodeTimeout:            http.StatusRequestTimeout,
	cellar.CodeWaking:             http.StatusServiceUnavailable,
	cellar.CodeUnavailable:        http.StatusServiceUnavailable,
	cellar.CodeDisabled:           http.StatusNotImplemented,
	cellar.CodeInternal:           http.StatusInternalServerError,
}
