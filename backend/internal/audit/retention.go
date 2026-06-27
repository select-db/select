package audit

import (
	"context"
	"log"
	"time"

	"backend/db/db_types"
)

const retentionSweepInterval = 24 * time.Hour

// partmanManaged reports whether pg_partman is installed — i.e. partitions and
// retention are managed in-DB. When false (on-prem without partman/cron), the
// in-app retention sweeper takes over. On error we assume managed, so an
// unrelated hiccup never triggers deletes.
func (l *Logger) partmanManaged() bool {
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()
	var ok bool
	if err := l.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_partman')`).Scan(&ok); err != nil {
		log.Printf("audit: partman check failed, retention sweeper disabled: %v", err)
		return true
	}
	return ok
}

// retentionLoop deletes events past the window — once at startup, then daily.
// Started only when pg_partman is absent (see Start); assumes on-prem scale, so
// it relies on a plain bounded DELETE rather than partition drops.
func (l *Logger) retentionLoop() {
	defer l.wg.Done()

	ticker := time.NewTicker(retentionSweepInterval)
	defer ticker.Stop()

	l.retentionSweep()
	for {
		select {
		case <-l.stop:
			return
		case <-ticker.C:
			l.retentionSweep()
		}
	}
}

func (l *Logger) retentionSweep() {
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()

	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("audit: retention sweep: %v", err)
		return
	}
	defer func() { _ = tx.Rollback() }()
	if err := applyStatementTimeout(ctx, tx); err != nil {
		log.Printf("audit: retention sweep: %v", err)
		return
	}

	cutoff := time.Now().Add(-l.retention)
	n, err := l.q.WithTx(tx).DeleteAuditEventsBefore(ctx, db_types.NewJSONNullTimeFromTime(cutoff))
	if err != nil {
		log.Printf("audit: retention sweep: %v", err)
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("audit: retention sweep commit: %v", err)
		return
	}
	if n > 0 {
		log.Printf("audit: retention swept %d events older than %s", n, cutoff.Format(time.RFC3339))
	}
}
