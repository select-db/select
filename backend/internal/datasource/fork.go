package datasource

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"backend/internal/authz"
)

type forkRequest struct {
	Name        string  `json:"name"`
	PointInTime string  `json:"at"`
	GrantTo     GrantTo `json:"grant_to"`
}

// ForkHandler copies a managed database into a new one; the source is never
// touched. Forking hands over all the data, so it needs manage on the source.
func ForkHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req forkRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		sourceID := r.PathValue("id")
		id, err := CreateManaged(r, req.Name, sourceID, req.PointInTime, req.GrantTo)
		if err != nil {
			OpenError(w, err, "managed fork", authz.ActorOf(r).WorkspaceID, sourceID)
			return
		}
		writeCreated(w, id, "sqlite")
	}
}

// checkPointInTime refuses a fork time outside the plan's window, or in the future.
func checkPointInTime(pointInTime string, retentionDays int) error {
	if pointInTime == "" {
		return nil
	}
	forkTime, err := time.Parse(time.RFC3339, pointInTime)
	if err != nil {
		return &Refusal{http.StatusBadRequest, "at must be an RFC 3339 time"}
	}
	if forkTime.After(time.Now()) || forkTime.Before(time.Now().AddDate(0, 0, -retentionDays)) {
		return &Refusal{http.StatusBadRequest, fmt.Sprintf("at must be within the last %d days", retentionDays)}
	}
	return nil
}
