// Package audit is the unified, append-only activity log: query, IAM, auth, and
// datasource events share one Event envelope and one writer with two lanes,
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
// contract: add actions, never rename or repurpose. The activity domains query
// and auth use a bare verb (executed, login). Lifecycle and privilege changes
// follow the Okta System Log convention of a dotted object.action hierarchy:
// lifecycle is <object>.lifecycle.<create|update|delete> (role.lifecycle.create,
// permission.lifecycle.delete; in datasource the domain is itself the object, so
// the action is a bare lifecycle.create); memberships are
// <container>.user_membership.<add|remove>; role grants are
// <subject>.role.<grant|revoke>.
const (
	// query (a permission block is ActionExecuted with StatusDenied, not its own action)
	ActionExecuted = "executed" // a statement ran (or was blocked) via the proxy

	// auth
	ActionLogin          = "login"
	ActionLoginFailed    = "login_failed"
	ActionTokenRefreshed = "token_refreshed"

	// iam
	ActionPermissionCreated    = "permission.lifecycle.create"
	ActionPermissionUpdated    = "permission.lifecycle.update"
	ActionPermissionDeleted    = "permission.lifecycle.delete"
	ActionRoleCreated          = "role.lifecycle.create"
	ActionRoleUpdated          = "role.lifecycle.update"
	ActionRoleDeleted          = "role.lifecycle.delete"
	ActionUserRoleGranted      = "user.role.grant"  // a role granted directly to a user
	ActionUserRoleRevoked      = "user.role.revoke" // a direct user-role grant revoked
	ActionWorkspaceUserAdded   = "workspace.user_membership.add"
	ActionWorkspaceUserRemoved = "workspace.user_membership.remove"
	ActionGroupCreated         = "group.lifecycle.create"
	ActionGroupUpdated         = "group.lifecycle.update"
	ActionGroupDeleted         = "group.lifecycle.delete"
	ActionGroupUserAdded       = "group.user_membership.add"
	ActionGroupUserRemoved     = "group.user_membership.remove"
	ActionGroupRoleGranted     = "group.role.grant"
	ActionGroupRoleRevoked     = "group.role.revoke"
	ActionWorkspaceCreated     = "workspace.lifecycle.create"
	ActionWorkspaceDeleted     = "workspace.lifecycle.delete"
	ActionWorkspaceLogoUpdated = "workspace.logo.update"
	ActionAPIKeyCreated        = "api_key.lifecycle.create"
	ActionAPIKeyRotated        = "api_key.lifecycle.rotate"
	ActionAPIKeyRevoked        = "api_key.lifecycle.revoke"
	ActionAPIKeySetRoles       = "api_key.role.set"

	// datasource (the domain is itself the object, so lifecycle actions are bare)
	ActionDatasourceCreated = "lifecycle.create"
	ActionDatasourceUpdated = "lifecycle.update"
	ActionDatasourceDeleted = "lifecycle.delete"
)

type Target struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Event is the log envelope; Payload holds domain-specific fields.
type Event struct {
	WorkspaceID string         `json:"workspace_id"`
	OccurredAt  time.Time      `json:"occurred_at"`           // event time (app-set)
	RecordedAt  time.Time      `json:"recorded_at,omitempty"` // persist time (DB-set, read-only)
	Domain      string         `json:"domain"`
	Action      string         `json:"action"`
	Principal   Principal      `json:"principal"`
	Target      *Target        `json:"target,omitempty"`
	Status      string         `json:"status"`
	Payload     map[string]any `json:"payload,omitempty"`
	ClientIP    string         `json:"client_ip,omitempty"`
}
