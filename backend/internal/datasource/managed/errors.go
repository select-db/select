package managed

import "net/http"

// Refusal is a request the caller can correct, answered with its HTTP status.
type Refusal struct {
	Status  int
	Message string
}

func (refusal *Refusal) Error() string { return refusal.Message }

var (
	ErrForbidden = &Refusal{http.StatusForbidden, "forbidden"}
	// ErrNotFound is a managed database the workspace does not have, or no longer serves.
	ErrNotFound = &Refusal{http.StatusNotFound, "datasource not found"}
)
