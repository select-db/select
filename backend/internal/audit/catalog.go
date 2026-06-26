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
	// query - Datastore Activity. The data plane: what was read/exported.
	QueryExecuted = Spec{
		Domain: DomainQuery, Action: ActionExecuted, Lane: LaneAsync, TargetType: "datasource",
		Doc: "a SQL query finished executing against a datasource via the proxy (status success|error)",
	}
	QueryDenied = Spec{
		Domain: DomainQuery, Action: ActionDenied, Lane: LaneAsync, TargetType: "datasource",
		Doc: "a query was blocked by the permission engine (status denied)",
	}
	QueryExported = Spec{
		Domain: DomainQuery, Action: ActionExported, Lane: LaneAsync, TargetType: "datasource",
		Doc: "a bulk export of a datasource was run",
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
	AuthLogout = Spec{
		Domain: DomainAuth, Action: ActionLogout, Lane: LaneAsync,
		Doc: "a session/refresh token was revoked",
	}

	// iam - Identity & Access Management. Privilege and account changes.
	PermissionUpserted = Spec{
		Domain: DomainIAM, Action: ActionPermissionUpserted, Lane: LaneOutbox, TargetType: "permission",
		Doc: "a permission rule was created or updated through the syncer",
	}
	PermissionDeleted = Spec{
		Domain: DomainIAM, Action: ActionPermissionDeleted, Lane: LaneOutbox, TargetType: "permission",
		Doc: "a permission rule was deleted",
	}
	RoleUpserted = Spec{
		Domain: DomainIAM, Action: ActionRoleUpserted, Lane: LaneOutbox, TargetType: "role",
		Doc: "a role was created or updated",
	}
	RoleDeleted = Spec{
		Domain: DomainIAM, Action: ActionRoleDeleted, Lane: LaneOutbox, TargetType: "role",
		Doc: "a role was deleted",
	}
	MemberAdded = Spec{
		Domain: DomainIAM, Action: ActionMemberAdded, Lane: LaneOutbox, TargetType: "user",
		Doc: "a user was added to a workspace",
	}
	MemberRemoved = Spec{
		Domain: DomainIAM, Action: ActionMemberRemoved, Lane: LaneOutbox, TargetType: "user",
		Doc: "a user was removed from a workspace",
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

	// datasource - connection lifecycle. Sensitive: DSNs can redirect data.
	DatasourceUpserted = Spec{
		Domain: DomainDatasource, Action: ActionDatasourceUpserted, Lane: LaneOutbox, TargetType: "datasource",
		Doc: "a datasource was created or its connection config changed",
	}
	DatasourceDeleted = Spec{
		Domain: DomainDatasource, Action: ActionDatasourceDeleted, Lane: LaneOutbox, TargetType: "datasource",
		Doc: "a datasource was deleted",
	}
)

// Catalog is the full declared vocabulary. Wired today: QueryExecuted,
// PermissionUpserted. The rest are reserved contract, emit sites land at their
// choke points incrementally.
var Catalog = []Spec{
	QueryExecuted, QueryDenied, QueryExported,
	AuthLogin, AuthLoginFailed, AuthTokenRefreshed, AuthLogout,
	PermissionUpserted, PermissionDeleted,
	RoleUpserted, RoleDeleted,
	MemberAdded, MemberRemoved,
	WorkspaceCreated, WorkspaceDeleted,
	APIKeyCreated, APIKeyRotated, APIKeyRevoked,
	DatasourceUpserted, DatasourceDeleted,
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

	// Query-only metrics; zero for other domains.
	DurationMs       int64
	ReturnedRowCount int64

	ClientIP string
}

// Emit builds the envelope from spec and dispatches to its lane. The only
// emission entry point, callers never touch Log/LogOutbox directly.
func Emit(ctx context.Context, spec Spec, r Record) error {
	if !registered[spec.Type()] {
		log.Printf("audit: Emit called with unregistered spec %q, add it to Catalog", spec.Type())
	}

	e := &Event{
		WorkspaceID:      r.WorkspaceID,
		Domain:           spec.Domain,
		Action:           spec.Action,
		Principal:        r.Principal,
		Status:           r.Status,
		Payload:          r.Payload,
		DurationMs:       r.DurationMs,
		ReturnedRowCount: r.ReturnedRowCount,
		ClientIP:         r.ClientIP,
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
