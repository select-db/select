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

// Categories partition the log physically.
const (
	CategoryQuery      = "query"
	CategoryAuth       = "auth"
	CategoryIAM        = "iam"
	CategoryDatasource = "datasource"
)

// Statuses.
const (
	StatusOK     = "ok"
	StatusError  = "error"
	StatusDenied = "denied"
)

// Principal kinds.
const (
	PrincipalUser   = "user"
	PrincipalAPIKey = "api_key"
)

// Event types (extend freely; a new type is just a string + an emit call).
const (
	TypeQueryExecuted      = "query.executed"
	TypeQueryDenied        = "query.denied"
	TypePermissionUpserted = "iam.permission.upserted"
)

// Principal is the snapshot of who acted and what they were allowed to do, as
// of the event. It is content-addressed: identical authz state hashes to the
// same value and is stored once in app.audit_principal_snapshot.
type Principal struct {
	Kind        string                 `json:"kind"`         // PrincipalUser | PrincipalAPIKey
	ID          string                 `json:"id"`           // user id or api-key id
	WorkspaceID string                 `json:"workspace_id"` // workspace in effect
	RoleIDs     []string               `json:"role_ids"`
	Permissions []core.PermissionEntry `json:"permissions"` // raw entries at event time
}

// canonical returns a deterministically-ordered copy so the hash is stable
// regardless of the order roles/permissions arrive in.
func (p Principal) canonical() Principal {
	roles := append([]string(nil), p.RoleIDs...)
	sort.Strings(roles)

	perms := append([]core.PermissionEntry(nil), p.Permissions...)
	sort.Slice(perms, func(i, j int) bool { return permKey(perms[i]) < permKey(perms[j]) })

	return Principal{
		Kind:        p.Kind,
		ID:          p.ID,
		WorkspaceID: p.WorkspaceID,
		RoleIDs:     roles,
		Permissions: perms,
	}
}

// JSON is the canonical snapshot body stored in audit_principal_snapshot.snapshot.
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

// Event is the unified log envelope. Payload holds category-specific fields.
type Event struct {
	WorkspaceID    string         `json:"workspace_id"`
	OccurredAt     time.Time      `json:"occurred_at"`
	Category       string         `json:"category"`
	Type           string         `json:"type"`
	Actor          Principal      `json:"actor"`
	Target         *Target        `json:"target,omitempty"`
	Status         string         `json:"status"`
	Payload        map[string]any `json:"payload,omitempty"`
	SQLFingerprint []byte         `json:"sql_fingerprint,omitempty"`
	DurationMs     int64          `json:"duration_ms,omitempty"`
	RowCount       int64          `json:"row_count,omitempty"`
	ClientIP       string         `json:"client_ip,omitempty"`
}

// Fingerprint hashes SQL for grouping/dedup without exposing the text in an
// index. Normalization (stripping literals/whitespace) is a follow-up; for now
// it hashes the raw statement.
func Fingerprint(sql string) []byte {
	sum := sha256.Sum256([]byte(sql))
	return sum[:]
}
