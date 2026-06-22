package patch

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"backend/internal/audit"
	"backend/internal/syncer/types"
)

// AuditConfig, when set on a Handler, makes Apply emit an audit event after a
// successful upsert. The before/after diff is derived generically from the
// fetched row and the upsert params, and the principal comes from the context
// (set by the sync handler), so an entity only declares the event spec + target.
type AuditConfig struct {
	Spec     audit.Spec
	TargetID string
}

// Handler describes the table-specific operations needed for LWW apply.
type Handler[Row any, Params any] struct {
	TableName string

	// Audit, when non-nil, emits an audit event after a successful upsert.
	Audit *AuditConfig

	// Fetch retrieves the current row by ID. Must return sql.ErrNoRows if absent.
	Fetch func(ctx context.Context) (Row, error)

	// UpdatedAt extracts the server updated_at from the fetched row.
	UpdatedAt func(Row) time.Time

	// DeletedAt extracts the server deleted_at from the fetched row. If non-nil and the returned
	// time is non-zero, an update/insert commit is rejected and the deleted row is sent back as Restored.
	DeletedAt func(Row) *time.Time

	// Restored converts a row to the server payload sent back to the client.
	Restored func(Row) (interface{}, error)

	// Merge builds the upsert params from the existing row and the commit payload.
	// isNew is true when the row does not yet exist on the server.
	Merge func(existing Row, isNew bool, payload map[string]any) (Params, error)

	// Upsert writes the merged params to the DB.
	Upsert func(ctx context.Context, params Params) error
}

// Apply runs last-write-wins for a single commit using h.
func Apply[Row any, Params any](ctx context.Context, c types.Commit, h Handler[Row, Params]) (bool, *types.RestoredItem, error) {
	payload, ok := c.Payload.(map[string]any)
	if !ok {
		return false, nil, fmt.Errorf("%s: payload must be an object", h.TableName)
	}

	existing, err := h.Fetch(ctx)
	isNew := errors.Is(err, sql.ErrNoRows)
	if err != nil && !isNew {
		return false, nil, fmt.Errorf("%s: fetch: %w", h.TableName, err)
	}

	// Server win: row is deleted; reject update and send deleted row so client applies delete locally.
	if !isNew && h.DeletedAt != nil {
		if t := h.DeletedAt(existing); t != nil && !t.IsZero() {
			serverPayload, err := h.Restored(existing)
			if err != nil {
				return false, nil, fmt.Errorf("%s: restore payload: %w", h.TableName, err)
			}
			return false, &types.RestoredItem{
				ObjectID:      c.ObjectID,
				TableName:     h.TableName,
				ServerPayload: serverPayload,
				UpdatedAt:     h.UpdatedAt(existing),
			}, nil
		}
	}

	// Clamp client timestamp to now
	clientTime := c.CreatedAt
	if now := time.Now(); clientTime.After(now) {
		clientTime = now
	}

	// Server win: send the authoritative row back so the client reverts.
	if !isNew && clientTime.Before(h.UpdatedAt(existing)) {
		serverPayload, err := h.Restored(existing)
		if err != nil {
			return false, nil, fmt.Errorf("%s: restore payload: %w", h.TableName, err)
		}
		return false, &types.RestoredItem{
			ObjectID:      c.ObjectID,
			TableName:     h.TableName,
			ServerPayload: serverPayload,
			UpdatedAt:     h.UpdatedAt(existing),
		}, nil
	}

	// Client wins: merge and upsert.
	params, err := h.Merge(existing, isNew, payload)
	if err != nil {
		return false, nil, fmt.Errorf("%s: merge: %w", h.TableName, err)
	}
	if err := h.Upsert(ctx, params); err != nil {
		return false, nil, fmt.Errorf("%s: upsert: %w", h.TableName, err)
	}

	if h.Audit != nil {
		var before any
		if !isNew {
			before = existing
		}
		// The event's workspace is the resource's (from the commit); the principal
		// is resolved for that same workspace.
		p := audit.ResolvePrincipal(ctx, c.WorkspaceID)
		if err := audit.Emit(ctx, h.Audit.Spec, audit.Record{
			WorkspaceID: c.WorkspaceID,
			Principal:   p,
			TargetID:    h.Audit.TargetID,
			Status:      audit.StatusSuccess,
			Payload:     audit.Diff(before, params),
		}); err != nil {
			// Best-effort: a logging failure must not fail the sync.
			log.Printf("audit: emit %s: %v", h.Audit.Spec.Type(), err)
		}
	}
	return true, nil, nil
}
