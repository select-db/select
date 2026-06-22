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

// Status partitions outcomes the way a SOC reads them: a failed login
// (StatusFailure, brute-force signal) is not an authorization block
// (StatusDenied, valid identity / missing permission) is not a system fault
// (StatusError).
const (
	StatusSuccess = "success"
	StatusFailure = "failure"
	StatusDenied  = "denied"
	StatusError   = "error"
)

// Actions pair with a domain; the full event id is domain.action. This is the
// closed vocabulary consumers' SOC/SIEM rules key on, so it's an external
// contract: add actions, never rename or repurpose. Single-subject domains
// (query, auth, datasource) use a bare verb; iam spans entities, so its actions
// are entity.verb.
const (
	// query
	ActionExecuted = "executed" // a statement ran via the proxy
	ActionDenied   = "denied"   // a query blocked by permissions
	ActionExported = "exported" // a bulk export/dump (exfiltration signal)

	// auth
	ActionLogin          = "login"
	ActionLoginFailed    = "login_failed"
	ActionTokenRefreshed = "token_refreshed"
	ActionLogout         = "logout"

	// iam
	ActionPermissionUpserted = "permission.upserted"
	ActionPermissionDeleted  = "permission.deleted"
	ActionRoleUpserted       = "role.upserted"
	ActionRoleDeleted        = "role.deleted"
	ActionMemberAdded        = "member.added"
	ActionMemberRemoved      = "member.removed"
	ActionWorkspaceCreated   = "workspace.created"
	ActionWorkspaceDeleted   = "workspace.deleted"
	ActionAPIKeyCreated      = "api_key.created"
	ActionAPIKeyRotated      = "api_key.rotated"
	ActionAPIKeyRevoked      = "api_key.revoked"

	// datasource
	ActionDatasourceUpserted = "upserted" // created or connection config changed
	ActionDatasourceDeleted  = "deleted"
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
