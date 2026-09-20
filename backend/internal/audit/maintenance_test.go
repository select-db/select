package audit_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"

	"backend/e2e"
	"backend/internal/audit"
)

func TestMain(m *testing.M) { e2e.Run(m) }

// requirePartman skips on the embedded Postgres, which has no pg_partman; CI
// runs these against a server that has it (TEST_PG_*).
func requirePartman(t *testing.T, db *sql.DB) {
	t.Helper()
	var has bool
	require.NoError(t, db.QueryRow(`SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_partman')`).Scan(&has))
	if !has {
		t.Skip("pg_partman not installed")
	}
}

func partitionExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var ok bool
	require.NoError(t, db.QueryRow(`SELECT to_regclass($1) IS NOT NULL`, name).Scan(&ok))
	return ok
}

// dropNewestPartition removes the furthest premade monthly partition, which
// maintenance is expected to recreate, and returns its name.
func dropNewestPartition(t *testing.T, db *sql.DB) string {
	t.Helper()
	var name string
	require.NoError(t, db.QueryRow(`
		SELECT c.oid::regclass::text FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
		WHERE i.inhparent = 'audit.event_query'::regclass AND c.relname NOT LIKE '%default'
		ORDER BY c.relname DESC LIMIT 1`).Scan(&name))
	_, err := db.Exec(`DROP TABLE ` + name)
	require.NoError(t, err)
	return name
}

func TestPartitionMaintenance_RecreatesPremadePartitions(t *testing.T) {
	db := e2e.NewDB(t)
	requirePartman(t, db)

	dropped := dropNewestPartition(t, db)

	require.NoError(t, audit.RunPartitionMaintenance(context.Background(), db))
	require.True(t, partitionExists(t, db, dropped), "maintenance did not recreate %s", dropped)
}

// Another instance holding the lock means this one skips the round, without error.
func TestPartitionMaintenance_SkipsWhenLockHeld(t *testing.T) {
	db := e2e.NewDB(t)
	requirePartman(t, db)

	ctx := context.Background()
	holder, err := db.Conn(ctx)
	require.NoError(t, err)
	defer func() { _ = holder.Close() }()
	var locked bool
	require.NoError(t, holder.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, audit.MaintenanceLockKey).Scan(&locked))
	require.True(t, locked)

	dropped := dropNewestPartition(t, db)

	require.NoError(t, audit.RunPartitionMaintenance(ctx, db))
	require.False(t, partitionExists(t, db, dropped), "maintenance ran despite the lock being held")
}
