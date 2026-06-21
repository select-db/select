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
	"crypto/sha256"
	"encoding/json"
	"sort"
	"time"

	core "github.com/selectDb/dialect/core"
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

// Principal kinds.
const (
	PrincipalUser   = "user"
	PrincipalAPIKey = "api_key"
)

// Actions are the specific events within a domain. The full event identity is
// domain + "." + action (e.g. query + executed). Extend freely; a new action is
// just a string + an emit call.
const (
	ActionExecuted           = "executed"            // domain=query
	ActionDenied             = "denied"              // domain=query
	ActionPermissionUpserted = "permission.upserted" // domain=iam
)

// Principal is the snapshot of who acted and what they were allowed to do, as
// of the event. It is content-addressed: identical authz state hashes to the
// same value and is stored once in audit.principal_snapshot.
type Principal struct {
	Type        string                 `json:"type"`         // PrincipalUser | PrincipalAPIKey
	ID          string                 `json:"id"`           // user id or api-key id
	Name        string                 `json:"name,omitempty"` // display name (user name/email, or key name)
	WorkspaceID string                 `json:"workspace_id"` // workspace in effect
	Roles       []Role                 `json:"roles"`        // roles with names, as of the event
	Permissions []core.PermissionEntry `json:"permissions"`  // raw entries at event time
}

// Role is a role id with its human-readable name, captured in the snapshot.
type Role struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// canonical returns a deterministically-ordered copy so the hash is stable
// regardless of the order roles/permissions arrive in.
func (p Principal) canonical() Principal {
	roles := append([]Role(nil), p.Roles...)
	sort.Slice(roles, func(i, j int) bool { return roles[i].ID < roles[j].ID })

	perms := append([]core.PermissionEntry(nil), p.Permissions...)
	sort.Slice(perms, func(i, j int) bool { return permKey(perms[i]) < permKey(perms[j]) })

	return Principal{
		Type:        p.Type,
		ID:          p.ID,
		Name:        p.Name,
		WorkspaceID: p.WorkspaceID,
		Roles:       roles,
		Permissions: perms,
	}
}

// JSON is the canonical snapshot body stored in principal_snapshot.snapshot.
func (p Principal) JSON() []byte {
	b, _ := json.Marshal(p.canonical())
	return b
}

// Hash is the content address (primary key) of the snapshot.
func (p Principal) Hash() []byte {
	sum := sha256.Sum256(p.JSON())
	return sum[:]
}

func permKey(e core.PermissionEntry) string {
	s := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	return s(e.DbInstanceID) + "|" + s(e.SchemaName) + "|" + s(e.TableName) + "|" +
		s(e.ColumnName) + "|" + e.Action + "|" + e.Effect + "|" + e.RoleName
}

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

