// Package audit is the unified, append-only activity log: query, IAM, auth, and
// datasource events share one Event envelope and one writer with two lanes —
// Log (best-effort async) and LogOutbox (durable transactional outbox).
// Encryption at rest is Tier 0 (storage-level), so payloads are plaintext JSONB.
package audit

import (
	"time"
)

// Domains are the LIST partition key.
const (
	DomainQuery      = "query"
	DomainAuth       = "auth"
	DomainIAM        = "iam"
	DomainDatasource = "datasource"
)

const (
	StatusSuccess = "success"
	StatusError   = "error"
	StatusDenied  = "denied"
)

// Actions pair with a domain; full event id is domain.action.
const (
	ActionExecuted           = "executed"            // domain=query
	ActionDenied             = "denied"              // domain=query
	ActionPermissionUpserted = "permission.upserted" // domain=iam
)

type Target struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Event is the log envelope; Payload holds domain-specific fields.
type Event struct {
	WorkspaceID      string         `json:"workspace_id"`
	OccurredAt       time.Time      `json:"occurred_at"`           // event time (app-set)
	RecordedAt       time.Time      `json:"recorded_at,omitempty"` // persist time (DB-set, read-only)
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
