package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"backend/db/db_types"
	"backend/db/generated"

	"github.com/sqlc-dev/pqtype"
)

type Options struct {
	BufferSize    int           // async channel capacity
	BatchSize     int           // rows per INSERT batch
	FlushInterval time.Duration // max time a buffered row waits before a flush
	OutboxPoll    time.Duration // how often the outbox is drained
}

const (
	defaultBufferSize    = 8192
	defaultBatchSize     = 200
	defaultFlushInterval = 250 * time.Millisecond
	writeTimeout         = 10 * time.Second
)

// Logger owns the async write pipeline. Call Stop only after the HTTP server has
// drained, so no Log call races the channel close.
type Logger struct {
	db            *sql.DB
	q             *generated.Queries
	ch            chan *Event
	batchSize     int
	flushInterval time.Duration
	outboxPoll    time.Duration

	wg       sync.WaitGroup
	stop     chan struct{}
	stopOnce sync.Once
}

func New(db *sql.DB, opts Options) *Logger {
	if opts.BufferSize <= 0 {
		opts.BufferSize = defaultBufferSize
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = defaultBatchSize
	}
	if opts.FlushInterval <= 0 {
		opts.FlushInterval = defaultFlushInterval
	}
	if opts.OutboxPoll <= 0 {
		opts.OutboxPoll = defaultOutboxPoll
	}
	return &Logger{
		db:            db,
		q:             generated.New(db),
		ch:            make(chan *Event, opts.BufferSize),
		batchSize:     opts.BatchSize,
		flushInterval: opts.FlushInterval,
		outboxPoll:    opts.OutboxPoll,
		stop:          make(chan struct{}),
	}
}

// Start launches the async writer and the outbox drainer.
func (l *Logger) Start() {
	if l == nil {
		return
	}
	l.wg.Add(2)
	go l.writeLoop()
	go l.outboxLoop()
}

// Log never blocks the caller: a full buffer drops the event (a query must not
// wait on audit logging).
func (l *Logger) Log(e *Event) {
	if l == nil || e == nil {
		return
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now()
	}
	select {
	case l.ch <- e:
	default:
		log.Printf("audit: buffer full, dropping %s.%s event", e.Domain, e.Action)
	}
}

// Stop flushes the remaining batch and drains the outbox. Call only after the
// HTTP server has stopped accepting requests.
func (l *Logger) Stop(ctx context.Context) error {
	if l == nil {
		return nil
	}
	l.stopOnce.Do(func() {
		close(l.stop) // stop outbox polling
		close(l.ch)   // writeLoop drains remaining events then exits
	})

	done := make(chan struct{})
	go func() { l.wg.Wait(); close(done) }()

	select {
	case <-done:
		// Bounded by the caller's shutdown budget; whatever doesn't flush in
		// time stays durably in the outbox and drains on the next start.
		return l.drainOutbox(ctx)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (l *Logger) writeLoop() {
	defer l.wg.Done()

	ticker := time.NewTicker(l.flushInterval)
	defer ticker.Stop()

	buf := make([]*Event, 0, l.batchSize)
	flush := func() {
		if len(buf) == 0 {
			return
		}
		if err := l.writeBatch(buf); err != nil {
			log.Printf("audit: write batch (%d events): %v", len(buf), err)
		}
		buf = buf[:0]
	}

	for {
		select {
		case e, ok := <-l.ch:
			if !ok {
				flush()
				return
			}
			buf = append(buf, e)
			if len(buf) >= l.batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// writeBatch upserts the distinct principal snapshots, then inserts the events,
// all in one transaction so the principal_hash FK is always satisfied.
func (l *Logger) writeBatch(events []*Event) error {
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()

	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	qtx := l.q.WithTx(tx)
	if err := applyStatementTimeout(ctx, tx); err != nil {
		return err
	}

	seen := make(map[string]struct{}, len(events))
	for _, e := range events {
		if err := upsertSnapshot(ctx, qtx, e, seen); err != nil {
			return err
		}
	}
	for _, e := range events {
		if err := insertEvent(ctx, qtx, e); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// applyStatementTimeout backstops lib/pq not honoring the Go context deadline at
// the socket level: Postgres aborts a server-blocked statement (lock wait,
// overload). SET LOCAL is tx-scoped, so it doesn't leak to the pool. Does not
// cover a dead-connection network hang (only TCP keepalive / pgx do).
func applyStatementTimeout(ctx context.Context, tx *sql.Tx) error {
	// ms; SET takes no bind params, so format the (internal int) value directly.
	_, err := tx.ExecContext(ctx, fmt.Sprintf("SET LOCAL statement_timeout = %d", writeTimeout.Milliseconds()))
	return err
}

func upsertSnapshot(ctx context.Context, q *generated.Queries, e *Event, seen map[string]struct{}) error {
	h := e.Principal.Hash()
	key := string(h)
	if _, ok := seen[key]; ok {
		return nil
	}
	seen[key] = struct{}{}
	return q.UpsertPrincipalSnapshot(ctx, generated.UpsertPrincipalSnapshotParams{
		SnapshotHash: h,
		WorkspaceID:  nullUUID(e.Principal.WorkspaceID),
		Snapshot:     jsonbRaw(e.Principal.JSON()),
	})
}

func insertEvent(ctx context.Context, q *generated.Queries, e *Event) error {
	payload := []byte("{}")
	if e.Payload != nil {
		if b, err := json.Marshal(e.Payload); err == nil {
			payload = b
		}
	}

	var target Target
	if e.Target != nil {
		target = *e.Target
	}

	return q.InsertAuditEvent(ctx, generated.InsertAuditEventParams{
		WorkspaceID:      nullUUID(e.WorkspaceID),
		OccurredAt:       db_types.NewJSONNullTimeFromTime(e.OccurredAt),
		Domain:           nullStr(e.Domain),
		Action:           nullStr(e.Action),
		PrincipalHash:    e.Principal.Hash(),
		PrincipalID:      nullUUID(e.Principal.ID),
		PrincipalType:    nullStr(e.Principal.Type),
		TargetType:       nullStr(target.Type),
		TargetID:         nullUUID(target.ID),
		TargetLabel:      nullStr(target.Label),
		Status:           nullStr(e.Status),
		Payload:          jsonbRaw(payload),
		DurationMs:       nullInt64(e.DurationMs),
		ReturnedRowCount: nullInt64(e.ReturnedRowCount),
		ClientIp:         nullInet(e.ClientIP),
	})
}

// Converters to the generated params (sqlc types are nullable); empty/zero → NULL.

func nullStr(s string) db_types.JSONNullString {
	if s == "" {
		return db_types.JSONNullString{}
	}
	return db_types.NewJSONNullString(s)
}

func nullUUID(s string) db_types.JSONNullUUID {
	if s == "" {
		return db_types.JSONNullUUID{}
	}
	v, err := db_types.NewJSONNullUUIDFromString(s)
	if err != nil {
		return db_types.JSONNullUUID{}
	}
	return v
}

func nullInt64(n int64) db_types.JSONNullInt64 {
	if n == 0 {
		return db_types.JSONNullInt64{}
	}
	return db_types.NewJSONNullInt64(n)
}

func nullInet(s string) db_types.JSONNullInet {
	if s == "" {
		return db_types.JSONNullInet{}
	}
	var inet pqtype.Inet
	if err := inet.Scan(s); err != nil {
		return db_types.JSONNullInet{}
	}
	return db_types.NewJSONNullInet(inet)
}

func jsonbRaw(b []byte) pqtype.NullRawMessage {
	return pqtype.NullRawMessage{RawMessage: b, Valid: len(b) > 0}
}
