package audit

import (
	"context"
	"log"
)

// Catalog is the single source of truth for what's logged; all emission goes
// through Emit, which takes a Spec + Record. To see what's logged, read Catalog.

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

var Catalog = []Spec{
	QueryExecuted,
	PermissionUpserted,
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
// emission entry point — callers never touch Log/LogOutbox directly.
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
