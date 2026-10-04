package managed

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
	"backend/internal/datasource/managed/cellarclient"

	"github.com/google/uuid"
)

const (
	reconcileEvery  = 10 * time.Minute
	reconcileFirst  = time.Minute
	reconcileTimeout = 5 * time.Minute
	// orphanAge is how long a database with no row is left alone: a create puts
	// the file on the cellar before it commits the row.
	orphanAge = 24 * time.Hour
	// maxPurges stops a run that wants to remove more, so a mistake cannot empty a cellar.
	maxPurges = 50
	// reconcileLockKey serializes the run across backend instances.
	reconcileLockKey = 0x63656c6c5f726563 // "cell_rec"
)

// StartReconciler runs Reconcile every reconcileEvery until ctx ends. It does
// nothing while managed databases are off.
func StartReconciler(ctx context.Context) {
	go func() {
		wait := reconcileFirst
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
			wait = reconcileEvery
			if cellarclient.URL == "" {
				continue
			}
			runCtx, cancel := context.WithTimeout(ctx, reconcileTimeout)
			result, err := Reconcile(runCtx, time.Now())
			cancel()
			if err != nil {
				log.Printf("WARNING: reconciler: %v", err)
			}
			if result.Purged > 0 || result.Failed > 0 || result.RowsDropped > 0 {
				log.Printf("reconciler: purged %d, failed %d, rows dropped %d", result.Purged, result.Failed, result.RowsDropped)
			}
		}
	}()
}

// ReconcileResult counts what one run did.
type ReconcileResult struct {
	Purged      int
	Failed      int
	RowsDropped int
	// CapHit is a run that stopped at maxPurges with more to purge.
	CapHit bool
}

// Reconcile removes from the cellar the databases nothing serves any more: the
// ones their row says are deleting, the ones of a deleted workspace, and the
// ones with no row for over orphanAge. It then drops the rows of deleting
// databases the cellar no longer holds.
//
// It decides one database at a time, from a row it has read. It never asks
// "delete everything not in this list", so a query that fails or answers short
// purges nothing: the run stops at the first error.
func Reconcile(ctx context.Context, now time.Time) (ReconcileResult, error) {
	var result ReconcileResult
	conn, err := db.GetDB().Conn(ctx)
	if err != nil {
		return result, err
	}
	defer func() { _ = conn.Close() }()
	var locked bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, int64(reconcileLockKey)).Scan(&locked); err != nil {
		return result, fmt.Errorf("taking lock: %w", err)
	}
	if !locked {
		return result, nil
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, int64(reconcileLockKey))
	}()

	stored, err := cellarclient.Inventory(ctx)
	if err != nil {
		return result, fmt.Errorf("listing the cellar: %w", err)
	}
	// A stable order, so a run stopped by the cap resumes where it will not repeat.
	slices.SortFunc(stored, func(a, b cellarclient.StoredDatabase) int { return strings.Compare(a.ID, b.ID) })
	held := make(map[uuid.UUID]bool, len(stored))

	for _, database := range stored {
		id, err := uuid.Parse(database.ID)
		if err != nil {
			continue
		}
		held[id] = true
		reason, purge, err := shouldPurge(ctx, id, database, now)
		if err != nil {
			return result, fmt.Errorf("reading %s: %w", id, err)
		}
		if !purge {
			continue
		}
		if result.Purged+result.Failed >= maxPurges {
			result.CapHit = true
			log.Printf("WARNING: reconciler: cap hit, %d purges in one run; the rest waits for the next", maxPurges)
			break
		}
		if err := cellarclient.Purge(ctx, database.ID); err != nil {
			log.Printf("reconciler: purge %s (%s): %v", id, reason, err)
			result.Failed++
			continue
		}
		result.Purged++
		log.Printf("reconciler: purged %s (%s)", id, reason)
		if err := dropRow(ctx, id); err != nil {
			return result, fmt.Errorf("dropping the row of %s: %w", id, err)
		}
	}
	if result.CapHit {
		return result, nil
	}

	// A deleting row the cellar does not hold has nothing left to purge.
	deleting, err := db.Queries.ListDeletingManagedDatasources(ctx, db_types.NewJSONNullString(cellarclient.CellarID))
	if err != nil {
		return result, err
	}
	for _, id := range deleting {
		if held[id] {
			continue
		}
		if err := dropRow(ctx, id); err != nil {
			return result, fmt.Errorf("dropping the row of %s: %w", id, err)
		}
		result.RowsDropped++
	}
	return result, nil
}

// shouldPurge decides on one database of this cellar, and says why.
func shouldPurge(ctx context.Context, id uuid.UUID, database cellarclient.StoredDatabase, now time.Time) (reason string, purge bool, err error) {
	row, err := db.Queries.GetManagedDatasourceToReconcile(ctx, id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// A zero time is a file the cellar could not date: not old enough to judge.
		if !database.ModifiedAt.IsZero() && now.Sub(database.ModifiedAt) >= orphanAge {
			return "no row for over " + orphanAge.String(), true, nil
		}
		return "", false, nil
	case err != nil:
		return "", false, err
	case row.CellarID.ValueOrEmpty() != cellarclient.CellarID:
		// Another cellar's database; its own cellar decides.
		return "", false, nil
	case row.State.ValueOrEmpty() == stateDeleting:
		return "deleting", true, nil
	case row.WorkspaceDeleted:
		return "its workspace is deleted", true, nil
	}
	return "", false, nil
}

func dropRow(ctx context.Context, id uuid.UUID) error {
	return db.Queries.DeleteManagedDatasourceRow(ctx, generated.DeleteManagedDatasourceRowParams{
		ID:       id,
		CellarID: db_types.NewJSONNullString(cellarclient.CellarID),
	})
}
