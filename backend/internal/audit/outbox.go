package audit

import (
	"context"
	"encoding/json"
	"log"
	"time"
)

const (
	defaultOutboxPoll = 2 * time.Second
	outboxBatch       = 100
)

// LogOutbox durably enqueues an event for crash-safety.
//
// TODO: not yet atomic with the audited mutation. It's a standalone INSERT
// after the mutation, not in its transaction (the syncer doesn't thread a tx
// through patch.Apply yet).
func (l *Logger) LogOutbox(ctx context.Context, e *Event) error {
	if l == nil || e == nil {
		return nil
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now()
	}
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return l.q.InsertAuditOutbox(ctx, jsonbRaw(body))
}

func (l *Logger) outboxLoop() {
	defer l.wg.Done()

	ticker := time.NewTicker(l.outboxPoll)
	defer ticker.Stop()

	for {
		select {
		case <-l.stop:
			return
		case <-ticker.C:
			if err := l.drainOutbox(context.Background()); err != nil {
				log.Printf("audit: outbox drain: %v", err)
			}
		}
	}
}

// drainOutbox moves queued events into audit.event in batches. Select + insert
// + delete share one transaction (FOR UPDATE SKIP LOCKED), so each row is moved
// exactly once and concurrent drainers don't collide.
func (l *Logger) drainOutbox(ctx context.Context) error {
	for {
		// Bound each batch so a stalled drain can't run unbounded (the callers
		// pass a deadline-less context for the periodic loop).
		batchCtx, cancel := context.WithTimeout(ctx, writeTimeout)
		n, err := l.drainOutboxBatch(batchCtx)
		cancel()
		if err != nil {
			return err
		}
		if n < outboxBatch {
			return nil
		}
	}
}

func (l *Logger) drainOutboxBatch(ctx context.Context) (int, error) {
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	qtx := l.q.WithTx(tx)
	if err := applyStatementTimeout(ctx, tx); err != nil {
		return 0, err
	}

	rows, err := qtx.GetAuditOutboxBatch(ctx, outboxBatch)
	if err != nil {
		return 0, err
	}

	var ids []int64
	var events []*Event
	for _, row := range rows {
		ids = append(ids, row.ID.Int64) // delete regardless so a poison row can't wedge the queue
		var e Event
		if err := json.Unmarshal(row.EventJson.RawMessage, &e); err != nil {
			log.Printf("audit: dropping unparseable outbox row %d: %v", row.ID.Int64, err)
			continue
		}
		events = append(events, &e)
	}

	if len(ids) == 0 {
		return 0, tx.Commit()
	}

	seen := make(map[string]struct{}, len(events))
	for _, e := range events {
		if err := upsertSnapshot(ctx, qtx, e, seen); err != nil {
			return 0, err
		}
		if err := insertEvent(ctx, qtx, e); err != nil {
			return 0, err
		}
	}

	if err := qtx.DeleteAuditOutbox(ctx, ids); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(ids), nil
}
