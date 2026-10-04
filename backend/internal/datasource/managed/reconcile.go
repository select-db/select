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
	"backend/internal/cellar"
	"backend/internal/datasource/managed/cellarclient"

	"github.com/google/uuid"
)

const (
	reconcileEvery   = 10 * time.Minute
	reconcileFirst   = time.Minute
	reconcileTimeout = 5 * time.Minute
	// orphanAge is how long a database with no row is left alone: a create puts
	// the file on the cellar before it commits the row.
	orphanAge = 24 * time.Hour
	// maxPurges stops a run that wants to remove more, so a mistake cannot empty a cellar.
	maxPurges = 50
	// maxFailuresInARow ends a run whose purges keep failing, so a cellar that is
	// down is not asked once per database.
	maxFailuresInARow = 3
	// reconcileLockKey serializes the run across backend instances.
	reconcileLockKey = 0x63656c6c5f726563 // "cell_rec"
)

// StartReconciler runs Reconcile every reconcileEvery until ctx ends. It is
// started once managed databases are on.
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
			runCtx, cancel := context.WithTimeout(ctx, reconcileTimeout)
			result, err := Reconcile(runCtx, time.Now())
			cancel()
			if err != nil {
				log.Printf("WARNING: reconciler: %v", err)
			}
			if result.Purged > 0 || result.Failed > 0 || result.RowsDropped > 0 {
				log.Printf("reconciler: purged %d, failed %d, rows dropped %d, sizes updated %d", result.Purged, result.Failed, result.RowsDropped, result.Resized)
			}
		}
	}()
}

// ReconcileResult counts what one run did.
type ReconcileResult struct {
	Purged      int
	Failed      int
	RowsDropped int
	// Resized is the live databases whose recorded size it brought up to date.
	Resized int
	// CapHit is a run that stopped at maxPurges with more to purge.
	CapHit bool
}

// Reconcile purges the databases nothing serves: deleting, of a deleted workspace,
// or with no row for orphanAge. It decides one at a time from a row it read, so
// a failing query or listing purges nothing.
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
	slices.SortFunc(stored, func(a, b cellar.StoredDatabase) int { return strings.Compare(a.ID, b.ID) })
	held := make(map[uuid.UUID]bool, len(stored))
	failedInARow := 0

	for _, database := range stored {
		id, err := uuid.Parse(database.ID)
		if err != nil {
			continue
		}
		held[id] = true
		reason, judgement, err := decide(ctx, id, database, now)
		if err != nil {
			return result, fmt.Errorf("reading %s: %w", id, err)
		}
		if judgement == live && !database.Cold && database.SizeBytes > 0 {
			// A cold database has no size to report: its row keeps the last one.
			if err := db.Queries.SetManagedDatasourceSize(ctx, generated.SetManagedDatasourceSizeParams{
				ID: id, CellarID: db_types.NewJSONNullString(cellarclient.CellarID), SizeBytes: db_types.NewJSONNullInt64(database.SizeBytes),
			}); err != nil {
				return result, fmt.Errorf("recording the size of %s: %w", id, err)
			}
			result.Resized++
		}
		if judgement != purge {
			continue
		}
		if result.Purged >= maxPurges {
			result.CapHit = true
			log.Printf("WARNING: reconciler: cap hit, %d purges in one run; the rest waits for the next", maxPurges)
			break
		}
		if err := cellarclient.Purge(ctx, database.ID); err != nil {
			log.Printf("reconciler: purge %s (%s): %v", id, reason, err)
			result.Failed++
			if failedInARow++; failedInARow >= maxFailuresInARow {
				return result, fmt.Errorf("%d purges failed in a row, last: %w", failedInARow, err)
			}
			continue
		}
		failedInARow = 0
		result.Purged++
		log.Printf("reconciler: purged %s (%s)", id, reason)
		if err := db.Queries.DeleteManagedDatasourceRow(ctx, generated.DeleteManagedDatasourceRowParams{
			ID: id, CellarID: db_types.NewJSONNullString(cellarclient.CellarID),
		}); err != nil {
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
		if err := db.Queries.DeleteManagedDatasourceRow(ctx, generated.DeleteManagedDatasourceRowParams{
			ID: id, CellarID: db_types.NewJSONNullString(cellarclient.CellarID),
		}); err != nil {
			return result, fmt.Errorf("dropping the row of %s: %w", id, err)
		}
		result.RowsDropped++
	}
	return result, nil
}

// verdict is what the reconciler does with a database of this cellar.
type verdict int

const (
	leave verdict = iota // not this cellar's to judge, or too young to judge
	live                 // served: kept, and its size recorded
	purge
)

// decide judges one database of this cellar, and says why when it purges.
func decide(ctx context.Context, id uuid.UUID, database cellar.StoredDatabase, now time.Time) (reason string, v verdict, err error) {
	row, err := db.Queries.GetManagedDatasourceToReconcile(ctx, id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// A zero time is a file the cellar could not date: not old enough to judge.
		if !database.ModifiedAt.IsZero() && now.Sub(database.ModifiedAt) >= orphanAge {
			return "no row for over " + orphanAge.String(), purge, nil
		}
		return "", leave, nil
	case err != nil:
		return "", leave, err
	case row.CellarID.ValueOrEmpty() != cellarclient.CellarID:
		// Another cellar's database; its own cellar decides.
		return "", leave, nil
	case row.State.ValueOrEmpty() == stateDeleting:
		return "deleting", purge, nil
	case row.WorkspaceDeleted:
		return "its workspace is deleted", purge, nil
	}
	return "", live, nil
}
