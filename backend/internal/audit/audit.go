// Package audit is the unified, append-only activity log for the SQL proxy:
// query executions, IAM changes, auth events, and datasource changes all share
// one Event envelope, one writer pipeline, and one set of tables.
//
// Two delivery lanes:
//   - Log:       best-effort async (high-volume query/auth events).
//   - LogOutbox: durable via a transactional outbox (security-critical
//     iam/datasource events).
//
// Encryption at rest is Tier 0 (storage-level / encrypted volume), so payloads
// are stored as plaintext JSONB and remain queryable. No KMS in the hot path.
package audit

import (
	"time"
)

// Domains are the coarse subsystem buckets and the LIST partition key.
const (
	DomainQuery      = "query"
	DomainAuth       = "auth"
	DomainIAM        = "iam"
	DomainDatasource = "datasource"
)

// Statuses.
const (
	StatusSuccess = "success"
	StatusError   = "error"
	StatusDenied  = "denied"
)

// Actions are the specific events within a domain. The full event identity is
// domain + "." + action (e.g. query + executed). Extend freely; a new action is
// just a string + an emit call.
const (
	ActionExecuted           = "executed"            // domain=query
	ActionDenied             = "denied"              // domain=query
	ActionPermissionUpserted = "permission.upserted" // domain=iam
)

// Target is the entity an event acted upon (nil for query/auth events).
type Target struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Event is the unified log envelope. Payload holds domain-specific fields.
type Event struct {
	WorkspaceID      string         `json:"workspace_id"`
	OccurredAt       time.Time      `json:"occurred_at"`           // when the event happened (set by the app)
	RecordedAt       time.Time      `json:"recorded_at,omitempty"` // when the row was persisted; assigned by the DB on insert (read-only)
	Domain           string         `json:"domain"`
	Action           string         `json:"action"`
	Principal        Principal      `json:"principal"`
	Target           *Target        `json:"target,omitempty"`
	Status           string         `json:"status"`
	Payload          map[string]any `json:"payload,omitempty"`
	DurationMs       int64          `json:"duration_ms,omitempty"`
	ReturnedRowCount int64          `json:"returned_row_count,omitempty"`
	ClientIP         string         `json:"client_ip,omitempty"`
}

