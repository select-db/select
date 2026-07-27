package patch

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"backend/internal/syncer/types"
)

// Handler describes the table-specific operations needed for LWW apply.
type Handler[Row any, Params any] struct {
	TableName string

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

// Result reports what Apply did so the caller can react — e.g. emit an audit
// event — without Apply itself knowing about those concerns. Before/After are the
// row state around the write and are set only when Applied (Before is nil on an
// insert); Created distinguishes an insert from an update so the caller can pick
// the right lifecycle event; Restored is set only when the server won and the
// client must revert.
type Result struct {
	Applied  bool
	Created  bool
	Restored *types.RestoredItem
	Before   any
	After    any
}

// Apply runs last-write-wins for a single commit using h.
func Apply[Row any, Params any](ctx context.Context, c types.Commit, h Handler[Row, Params]) (Result, error) {
	payload, ok := c.Payload.(map[string]any)
	if !ok {
		return Result{}, fmt.Errorf("%s: payload must be an object", h.TableName)
	}

	existing, err := h.Fetch(ctx)
	isNew := errors.Is(err, sql.ErrNoRows)
	if err != nil && !isNew {
		return Result{}, fmt.Errorf("%s: fetch: %w", h.TableName, err)
	}

	// Server win: row is deleted; reject update and send deleted row so client applies delete locally.
	if !isNew && h.DeletedAt != nil {
		if t := h.DeletedAt(existing); t != nil && !t.IsZero() {
			serverPayload, err := h.Restored(existing)
			if err != nil {
				return Result{}, fmt.Errorf("%s: restore payload: %w", h.TableName, err)
			}
			return Result{Restored: &types.RestoredItem{
				ObjectID:      c.ObjectID,
				TableName:     h.TableName,
				ServerPayload: serverPayload,
				UpdatedAt:     h.UpdatedAt(existing),
			}}, nil
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
			return Result{}, fmt.Errorf("%s: restore payload: %w", h.TableName, err)
		}
		return Result{Restored: &types.RestoredItem{
			ObjectID:      c.ObjectID,
			TableName:     h.TableName,
			ServerPayload: serverPayload,
			UpdatedAt:     h.UpdatedAt(existing),
		}}, nil
	}

	// Client wins: merge and upsert.
	params, err := h.Merge(existing, isNew, payload)
	if err != nil {
		return Result{}, fmt.Errorf("%s: merge: %w", h.TableName, err)
	}
	if err := h.Upsert(ctx, params); err != nil {
		return Result{}, fmt.Errorf("%s: upsert: %w", h.TableName, err)
	}

	var before any
	if !isNew {
		before = existing
	}
	return Result{Applied: true, Created: isNew, Before: before, After: params}, nil
}
