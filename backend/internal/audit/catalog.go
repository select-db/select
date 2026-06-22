package audit

import (
	"context"
	"log"
)

// This file is the single source of truth for what the audit log records: every
// event is declared once in Catalog, and all emission goes through Emit. To see
// "what is logged and when", read Catalog. Emit sites never build envelopes or
// choose lanes themselves — they name a Spec and hand over the data.

// Lane selects the delivery guarantee for an event.
type Lane int

const (
	// LaneAsync is best-effort, off the request hot path (high-volume query/auth).
	LaneAsync Lane = iota
	// LaneOutbox is durable via the transactional outbox (security-critical
	// iam/datasource changes).
	LaneOutbox
)

// Spec is the fixed definition of an event type: its identity, delivery lane,
// the kind of target it acts on, and a human description of when it fires.
type Spec struct {
	Domain     string
	Action     string
	Lane       Lane
	TargetType string // "" when the event has no target
	Doc        string // when it fires (human-readable; powers the generated map)
}

// Type is the full event identity, domain.action.
func (s Spec) Type() string { return s.Domain + "." + s.Action }

// The catalog. Adding an event = adding a Spec here + an Emit call at the
// relevant choke point (query sink, patch.Apply, or the auth middleware).
var (
	QueryExecuted = Spec{
		Domain: DomainQuery, Action: ActionExecuted, Lane: LaneAsync, TargetType: "datasource",
		Doc: "a SQL query finished executing against a datasource via the proxy (status success|error)",
	}
	PermissionUpserted = Spec{
		Domain: DomainIAM, Action: ActionPermissionUpserted, Lane: LaneOutbox, TargetType: "permission",
		Doc: "a permission rule was created or updated through the syncer",
	}
)

// Catalog is the complete set of event types the system emits.
var Catalog = []Spec{
	QueryExecuted,
	PermissionUpserted,
}

// registered indexes the catalog so Emit can flag an unregistered spec in dev.
var registered = func() map[string]bool {
	m := make(map[string]bool, len(Catalog))
	for _, s := range Catalog {
		m[s.Type()] = true
	}
	return m
}()

// Record is the per-event data an emit site provides. The envelope (domain,
// action, target type, lane) comes from the Spec, not the caller.
type Record struct {
	WorkspaceID string
	Principal   Principal
	TargetID    string // bound to Spec.TargetType
	TargetLabel string
	Status      string // StatusSuccess | StatusError | StatusDenied
	Payload     map[string]any

	// Query-only metrics; zero for other domains.
	DurationMs       int64
	ReturnedRowCount int64

	ClientIP string
}

// Emit records an event: it builds the envelope from spec, attaches the record,
// and dispatches to the lane the spec declares. This is the only emission entry
// point — callers never touch Log/LogOutbox or the Event envelope directly.
func Emit(ctx context.Context, spec Spec, r Record) error {
	if !registered[spec.Type()] {
		log.Printf("audit: Emit called with unregistered spec %q — add it to Catalog", spec.Type())
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

// principalCtxKey carries the request's audit principal so emit sites deep in a
// call chain (e.g. syncer apply paths) can attach it without threading it
// through every signature — ctx is already passed everywhere.
type principalCtxKey struct{}

// ContextWithPrincipal stashes the principal for downstream emit sites.
func ContextWithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalCtxKey{}, p)
}

// PrincipalFromContext returns the principal set by ContextWithPrincipal, or a
// zero Principal if none.
func PrincipalFromContext(ctx context.Context) Principal {
	p, _ := ctx.Value(principalCtxKey{}).(Principal)
	return p
}
