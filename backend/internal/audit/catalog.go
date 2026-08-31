package audit

import (
	"context"
	"log"
)

// Catalog declares the full audit event vocabulary, the external contract a
// consumer's SOC/SIEM keys on. All emission goes through Emit(Spec, Record); to
// see what can be logged, read Catalog. A spec is part of the contract whether
// or not its emit site is wired yet (see "wired today" below); the rest are
// reserved so the taxonomy is fixed up front instead of churning the contract.

type Lane int

const (
	LaneAsync  Lane = iota // best-effort, off the hot path (query/auth)
	LaneOutbox             // durable via the transactional outbox (iam/datasource)
)

type Spec struct {
	Domain     string
	Action     string
	Lane       Lane
	TargetType string // "" when the event has no target
	Doc        string // when it fires
}

func (s Spec) Type() string { return s.Domain + "." + s.Action }

// Adding an event = a Spec here + an Emit call at the relevant choke point.
// Lane rationale: the high-volume data plane (query, auth) is best-effort async;
// control-plane mutations (iam, datasource) go through the durable outbox so a
// crash can't lose a privilege or config change.
var (
	// query - Datastore Activity. The data plane: what was read.
	QueryExecuted = Spec{
		Domain: DomainQuery, Action: ActionExecuted, Lane: LaneAsync, TargetType: "datasource",
		// A permission block is a denied outcome of the same attempt, not a
		// separate event: users routinely fumble into fields they can't read, so
		// it's a filterable status, not a standalone security signal.
		Doc: "a SQL query was run against a datasource (status success|error|denied)",
	}

	// auth - Authentication. Proving identity; no target (the principal is the subject).
	AuthLogin = Spec{
		Domain: DomainAuth, Action: ActionLogin, Lane: LaneAsync,
		Doc: "a principal authenticated and a token was issued",
	}
	AuthLoginFailed = Spec{
		Domain: DomainAuth, Action: ActionLoginFailed, Lane: LaneAsync,
		Doc: "an authentication attempt was rejected (status failure)",
	}
	AuthTokenRefreshed = Spec{
		Domain: DomainAuth, Action: ActionTokenRefreshed, Lane: LaneAsync,
		Doc: "an access token was reissued from a refresh token",
	}

	// iam - Identity & Access Management. Privilege and account changes.
	PermissionCreated = Spec{
		Domain: DomainIAM, Action: ActionPermissionCreated, Lane: LaneOutbox, TargetType: "permission",
		Doc: "a permission rule was created through the syncer",
	}
	PermissionUpdated = Spec{
		Domain: DomainIAM, Action: ActionPermissionUpdated, Lane: LaneOutbox, TargetType: "permission",
		Doc: "a permission rule was updated through the syncer",
	}
	PermissionDeleted = Spec{
		Domain: DomainIAM, Action: ActionPermissionDeleted, Lane: LaneOutbox, TargetType: "permission",
		Doc: "a permission rule was deleted",
	}
	RoleCreated = Spec{
		Domain: DomainIAM, Action: ActionRoleCreated, Lane: LaneOutbox, TargetType: "role",
		Doc: "a role was created",
	}
	RoleUpdated = Spec{
		Domain: DomainIAM, Action: ActionRoleUpdated, Lane: LaneOutbox, TargetType: "role",
		Doc: "a role was updated",
	}
	RoleDeleted = Spec{
		Domain: DomainIAM, Action: ActionRoleDeleted, Lane: LaneOutbox, TargetType: "role",
		Doc: "a role was deleted",
	}
	UserRoleGranted = Spec{
		Domain: DomainIAM, Action: ActionUserRoleGranted, Lane: LaneOutbox, TargetType: "user",
		Doc: "a role was granted directly to a user (user_to_role)",
	}
	UserRoleRevoked = Spec{
		Domain: DomainIAM, Action: ActionUserRoleRevoked, Lane: LaneOutbox, TargetType: "user",
		Doc: "a role granted directly to a user was revoked",
	}
	WorkspaceUserAdded = Spec{
		Domain: DomainIAM, Action: ActionWorkspaceUserAdded, Lane: LaneOutbox, TargetType: "user",
		Doc: "a user was added to a workspace",
	}
	WorkspaceUserRemoved = Spec{
		Domain: DomainIAM, Action: ActionWorkspaceUserRemoved, Lane: LaneOutbox, TargetType: "user",
		Doc: "a user was removed from a workspace",
	}
	GroupCreated = Spec{
		Domain: DomainIAM, Action: ActionGroupCreated, Lane: LaneOutbox, TargetType: "group",
		Doc: "a group was created",
	}
	GroupUpdated = Spec{
		Domain: DomainIAM, Action: ActionGroupUpdated, Lane: LaneOutbox, TargetType: "group",
		Doc: "a group was updated (e.g. renamed)",
	}
	GroupDeleted = Spec{
		Domain: DomainIAM, Action: ActionGroupDeleted, Lane: LaneOutbox, TargetType: "group",
		Doc: "a group was deleted",
	}
	GroupUserAdded = Spec{
		Domain: DomainIAM, Action: ActionGroupUserAdded, Lane: LaneOutbox, TargetType: "group",
		Doc: "a user was added to a group",
	}
	GroupUserRemoved = Spec{
		Domain: DomainIAM, Action: ActionGroupUserRemoved, Lane: LaneOutbox, TargetType: "group",
		Doc: "a user was removed from a group",
	}
	GroupRoleGranted = Spec{
		Domain: DomainIAM, Action: ActionGroupRoleGranted, Lane: LaneOutbox, TargetType: "group",
		Doc: "a role was granted to a group (grants the role to every user in the group)",
	}
	GroupRoleRevoked = Spec{
		Domain: DomainIAM, Action: ActionGroupRoleRevoked, Lane: LaneOutbox, TargetType: "group",
		Doc: "a role was revoked from a group",
	}
	WorkspaceCreated = Spec{
		Domain: DomainIAM, Action: ActionWorkspaceCreated, Lane: LaneOutbox, TargetType: "workspace",
		Doc: "a workspace was created",
	}
	WorkspaceDeleted = Spec{
		Domain: DomainIAM, Action: ActionWorkspaceDeleted, Lane: LaneOutbox, TargetType: "workspace",
		Doc: "a workspace was deleted",
	}
	APIKeyCreated = Spec{
		Domain: DomainIAM, Action: ActionAPIKeyCreated, Lane: LaneOutbox, TargetType: "api_key",
		Doc: "an API key was created",
	}
	APIKeyRotated = Spec{
		Domain: DomainIAM, Action: ActionAPIKeyRotated, Lane: LaneOutbox, TargetType: "api_key",
		Doc: "an API key was rotated",
	}
	APIKeyRevoked = Spec{
		Domain: DomainIAM, Action: ActionAPIKeyRevoked, Lane: LaneOutbox, TargetType: "api_key",
		Doc: "an API key was revoked",
	}
	APIKeySetRoles = Spec{
		Domain: DomainIAM, Action: ActionAPIKeySetRoles, Lane: LaneOutbox, TargetType: "api_key",
		Doc: "an API key's bound roles were replaced",
	}

	// datasource - connection lifecycle. Sensitive: DSNs can redirect data.
	DatasourceCreated = Spec{
		Domain: DomainDatasource, Action: ActionDatasourceCreated, Lane: LaneOutbox, TargetType: "datasource",
		Doc: "a datasource was created",
	}
	DatasourceUpdated = Spec{
		Domain: DomainDatasource, Action: ActionDatasourceUpdated, Lane: LaneOutbox, TargetType: "datasource",
		Doc: "a datasource's connection config was changed",
	}
	DatasourceDeleted = Spec{
		Domain: DomainDatasource, Action: ActionDatasourceDeleted, Lane: LaneOutbox, TargetType: "datasource",
		Doc: "a datasource was deleted",
	}
)

// Catalog is the full declared vocabulary. Every spec here has a live emit site
// (a denied query rides QueryExecuted with status=denied, not a spec of its own).
var Catalog = []Spec{
	QueryExecuted,
	AuthLogin, AuthLoginFailed, AuthTokenRefreshed,
	PermissionCreated, PermissionUpdated, PermissionDeleted,
	RoleCreated, RoleUpdated, RoleDeleted, UserRoleGranted, UserRoleRevoked,
	WorkspaceUserAdded, WorkspaceUserRemoved,
	GroupCreated, GroupUpdated, GroupDeleted,
	GroupUserAdded, GroupUserRemoved,
	GroupRoleGranted, GroupRoleRevoked,
	WorkspaceCreated, WorkspaceDeleted,
	APIKeyCreated, APIKeyRotated, APIKeyRevoked, APIKeySetRoles,
	DatasourceCreated, DatasourceUpdated, DatasourceDeleted,
}

// registered lets Emit flag an unregistered spec in dev.
var registered = func() map[string]bool {
	m := make(map[string]bool, len(Catalog))
	for _, s := range Catalog {
		m[s.Type()] = true
	}
	return m
}()

// Record is the per-event data a caller provides; the envelope comes from Spec.
type Record struct {
	WorkspaceID string
	Principal   Principal
	TargetID    string // bound to Spec.TargetType
	TargetLabel string
	Status      string
	Payload     map[string]any

	ClientIP string
}

// Emit builds the envelope from spec and dispatches to its lane. The only
// emission entry point, callers never touch Log/LogOutbox directly.
func Emit(ctx context.Context, spec Spec, r Record) error {
	if !registered[spec.Type()] {
		log.Printf("audit: Emit called with unregistered spec %q, add it to Catalog", spec.Type())
	}

	e := &Event{
		WorkspaceID: r.WorkspaceID,
		Domain:      spec.Domain,
		Action:      spec.Action,
		Principal:   r.Principal,
		Status:      r.Status,
		Payload:     r.Payload,
		ClientIP:    r.ClientIP,
	}
	if spec.TargetType != "" {
		e.Target = &Target{Type: spec.TargetType, ID: r.TargetID, Label: r.TargetLabel}
	}

	if spec.Lane == LaneOutbox {
		return LogOutbox(ctx, e)
	}
	Log(e)
	return nil
}

// EmitAction is the best-effort emit for handler-initiated events: it resolves
// the principal from ctx (the api keystone installs the resolver) and logs rather
// than returns on failure, so a logging error never fails the request. The caller
// fills r.WorkspaceID, TargetID/Label, Status, and a curated (secret-free!)
// Payload. For mutation events with a before/after diff, use EmitChange.
func EmitAction(ctx context.Context, spec Spec, r Record) {
	r.Principal = ResolvePrincipal(ctx, r.WorkspaceID)
	if err := Emit(ctx, spec, r); err != nil {
		log.Printf("audit: emit %s: %v", spec.Type(), err)
	}
}

// EmitDenied records a permission denial from a handler's authz gate. A zero
// spec (a read path with no action to attribute) is skipped; targetID may be ""
// when the denied resource isn't known at gate time.
func EmitDenied(ctx context.Context, spec Spec, workspaceID, targetID string) {
	if spec.Action == "" {
		return
	}
	EmitAction(ctx, spec, Record{WorkspaceID: workspaceID, TargetID: targetID, Status: StatusDenied})
}

// EmitChange is EmitAction for the mutation paths (upsert via patch.Apply,
// hand-written deletes): the payload is a before/after diff. Either side may be
// nil — an insert has no before, a delete has no after.
func EmitChange(ctx context.Context, spec Spec, workspaceID, targetID string, before, after any) {
	EmitAction(ctx, spec, Record{
		WorkspaceID: workspaceID,
		TargetID:    targetID,
		Status:      StatusSuccess,
		Payload:     Diff(before, after),
	})
}
