package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"sync"
	"time"

	"backend/db/db_types"
	"backend/db/generated"

	"github.com/sqlc-dev/pqtype"
)

// Options tunes the writer. Zero values fall back to the defaults below.
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
	defaultOutboxPoll    = 2 * time.Second
	writeTimeout         = 10 * time.Second
	outboxBatch          = 100
)

// Logger owns the async write pipeline. Construct with New, then Start; call
// Stop after the HTTP server has drained so no Log calls race the channel close.
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

// Start launches the async writer and the outbox drainer. (Partition lifecycle
// is handled in-database by pg_partman + pg_cron, not by the app.)
func (l *Logger) Start() {
	if l == nil {
		return
	}
	l.wg.Add(2)
	go l.writeLoop()
	go l.outboxLoop()
}

// Log enqueues an event on the best-effort async lane. It never blocks the
// caller: if the buffer is full the event is dropped (and logged), because a
// user's query must never wait on audit logging.
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

// LogOutbox durably enqueues an event via the transactional outbox. Use for
// security-critical events that must survive a crash.
//
// NOTE: ideally the outbox INSERT shares the caller's transaction with the
// audited mutation (true atomicity). Until the syncer threads a *sql.Tx through
// patch.Apply, this performs a standalone INSERT immediately after the mutation
// — durable and drained, but not yet atomic with it. Swap to a tx-aware call
// once that seam exists.
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

// Stop closes the intake, flushes the remaining batch, and does a final outbox
// drain. Must be called after the HTTP server has stopped accepting requests.
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
		return l.drainOutbox(context.WithoutCancel(ctx))
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
		n, err := l.drainOutboxBatch(ctx)
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

// Converters to the generated params. schema.sql leaves audit columns nullable,
// so sqlc generates JSONNull* / nullable types; these map empty/zero to NULL.

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
