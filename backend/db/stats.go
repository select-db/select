package db

import (
	"context"
	"time"

	"github.com/selectDb/toolkit/cache"
)

// PoolStats is the backend's pool of connections, as database/sql counts it.
type PoolStats struct {
	Max    int   `json:"max"`
	Open   int   `json:"open"`
	InUse  int   `json:"inUse"`
	Idle   int   `json:"idle"`
	Waits  int64 `json:"waits"` // connections asked for when none was free, since the start
	WaitMs int64 `json:"waitMs"`
}

// PostgresStats is what the database says about itself, from pg_stat_database.
type PostgresStats struct {
	Backends  int    `json:"backends"`
	Commits   int64  `json:"commits"`
	Rollbacks int64  `json:"rollbacks"`
	BlksRead  int64  `json:"blksRead"`
	BlksHit   int64  `json:"blksHit"`
	Deadlocks int64  `json:"deadlocks"`
	TempBytes int64  `json:"tempBytes"`
	SizeBytes int64  `json:"sizeBytes"`
	Error     string `json:"error,omitempty"`
}

// LockStats is the sessions of this database waiting for a lock.
type LockStats struct {
	Waiting       int    `json:"waiting"`
	LongestWaitMs int64  `json:"longestWaitMs"` // since the session's last state change
	Error         string `json:"error,omitempty"`
}

// TableStats is a table among the largest. Dead against live tuples is the bloat:
// autovacuum is behind when it grows.
type TableStats struct {
	Name           string `json:"name"`
	Bytes          int64  `json:"bytes"` // with its indexes and its toast
	LiveTuples     int64  `json:"liveTuples"`
	DeadTuples     int64  `json:"deadTuples"`
	SeqScans       int64  `json:"seqScans"`
	IdxScans       int64  `json:"idxScans"`
	LastAutovacuum int64  `json:"lastAutovacuum,omitempty"` // unix seconds
}

// TablesStats is the largest tables, or why they could not be read.
type TablesStats struct {
	Largest []TableStats `json:"largest,omitempty"`
	Error   string       `json:"error,omitempty"`
}

// ReplicationStats is the standbys this role can see: without pg_read_all_stats, none.
type ReplicationStats struct {
	Replicas int    `json:"replicas"`
	MaxLagMs int64  `json:"maxLagMs"`
	Error    string `json:"error,omitempty"`
}

// Statement is a normalised query of pg_stat_statements: placeholders, no values.
type Statement struct {
	Query   string  `json:"query"`
	Calls   int64   `json:"calls"`
	TotalMs float64 `json:"totalMs"`
	MeanMs  float64 `json:"meanMs"`
	Rows    int64   `json:"rows"`
}

// StatementsStats is the queries that took the most time, when the extension is there.
type StatementsStats struct {
	Installed bool        `json:"installed"`
	Top       []Statement `json:"top,omitempty"`
	Error     string      `json:"error,omitempty"`
}

// StatsReport is the "db" section of /debug/stats.
type StatsReport struct {
	Pool        *PoolStats        `json:"pool,omitempty"`
	Postgres    *PostgresStats    `json:"postgres,omitempty"`
	Locks       *LockStats        `json:"locks,omitempty"`
	Tables      *TablesStats      `json:"tables,omitempty"`
	Replication *ReplicationStats `json:"replication,omitempty"`
	Statements  *StatementsStats  `json:"statements,omitempty"`
}

const (
	// statsTTL is how long a reading is reused, however many ask.
	statsTTL = 15 * time.Second
	// statsDeadline bounds all the reads of one scrape, so a slow database cannot hold it.
	statsDeadline = 4 * time.Second
)

var readings = cache.New(cache.Options{TTL: statsTTL})

// reading is read(ctx) kept under key for statsTTL.
func reading[T any](ctx context.Context, key string, read func(context.Context) T) T {
	value, _ := readings.GetOrCreate(key, func() (any, error) { return read(ctx), nil })
	return value.(T)
}

// Stats is read only when /debug/stats is: nothing is counted or queried otherwise.
func Stats() any {
	if conn == nil {
		return StatsReport{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), statsDeadline)
	defer cancel()
	pool := conn.Stats()
	postgres, locks := reading(ctx, "postgres", readPostgres), reading(ctx, "locks", readLocks)
	tables, replication := reading(ctx, "tables", readTables), reading(ctx, "replication", readReplication)
	statements := reading(ctx, "statements", readStatements)
	return StatsReport{
		Pool: &PoolStats{
			Max:    pool.MaxOpenConnections,
			Open:   pool.OpenConnections,
			InUse:  pool.InUse,
			Idle:   pool.Idle,
			Waits:  pool.WaitCount,
			WaitMs: pool.WaitDuration.Milliseconds(),
		},
		Postgres:    &postgres,
		Locks:       &locks,
		Tables:      &tables,
		Replication: &replication,
		Statements:  &statements,
	}
}

func readPostgres(ctx context.Context) PostgresStats {
	var s PostgresStats
	err := conn.QueryRowContext(ctx, `
		SELECT numbackends, xact_commit, xact_rollback, blks_read, blks_hit, deadlocks, temp_bytes,
		       pg_database_size(datname)
		FROM pg_stat_database WHERE datname = current_database()`).
		Scan(&s.Backends, &s.Commits, &s.Rollbacks, &s.BlksRead, &s.BlksHit, &s.Deadlocks, &s.TempBytes, &s.SizeBytes)
	if err != nil {
		return PostgresStats{Error: err.Error()}
	}
	return s
}

func readLocks(ctx context.Context) LockStats {
	var l LockStats
	err := conn.QueryRowContext(ctx, `
		SELECT count(*) FILTER (WHERE wait_event_type = 'Lock'),
		       coalesce(max(extract(epoch FROM now() - state_change) * 1000)
		                FILTER (WHERE wait_event_type = 'Lock'), 0)::bigint
		FROM pg_stat_activity WHERE datname = current_database()`).Scan(&l.Waiting, &l.LongestWaitMs)
	if err != nil {
		return LockStats{Error: err.Error()}
	}
	return l
}

func readTables(ctx context.Context) TablesStats {
	rows, err := conn.QueryContext(ctx, `
		SELECT relname, pg_total_relation_size(relid), n_live_tup, n_dead_tup,
		       coalesce(seq_scan, 0), coalesce(idx_scan, 0),
		       coalesce(extract(epoch FROM last_autovacuum)::bigint, 0)
		FROM pg_stat_user_tables ORDER BY pg_total_relation_size(relid) DESC LIMIT 8`)
	if err != nil {
		return TablesStats{Error: err.Error()}
	}
	defer func() { _ = rows.Close() }()
	var out TablesStats
	for rows.Next() {
		var t TableStats
		if err := rows.Scan(&t.Name, &t.Bytes, &t.LiveTuples, &t.DeadTuples, &t.SeqScans, &t.IdxScans, &t.LastAutovacuum); err != nil {
			return TablesStats{Error: err.Error()}
		}
		out.Largest = append(out.Largest, t)
	}
	if err := rows.Err(); err != nil {
		return TablesStats{Error: err.Error()}
	}
	return out
}

func readReplication(ctx context.Context) ReplicationStats {
	var r ReplicationStats
	err := conn.QueryRowContext(ctx, `
		SELECT count(*), coalesce(max(extract(epoch FROM replay_lag) * 1000), 0)::bigint
		FROM pg_stat_replication`).Scan(&r.Replicas, &r.MaxLagMs)
	if err != nil {
		return ReplicationStats{Error: err.Error()}
	}
	return r
}

// readStatements reads pg_stat_statements when it is installed in this database.
func readStatements(ctx context.Context) StatementsStats {
	var installed bool
	if err := conn.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements')`).Scan(&installed); err != nil {
		return StatementsStats{Error: err.Error()}
	}
	if !installed {
		return StatementsStats{}
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT left(query, 300), calls, total_exec_time, mean_exec_time, rows
		FROM pg_stat_statements WHERE dbid = (SELECT oid FROM pg_database WHERE datname = current_database())
		ORDER BY total_exec_time DESC LIMIT 8`)
	if err != nil {
		// installed but not loaded (shared_preload_libraries), or not readable by this role
		return StatementsStats{Installed: true, Error: err.Error()}
	}
	defer func() { _ = rows.Close() }()
	out := StatementsStats{Installed: true}
	for rows.Next() {
		var st Statement
		if err := rows.Scan(&st.Query, &st.Calls, &st.TotalMs, &st.MeanMs, &st.Rows); err != nil {
			return StatementsStats{Installed: true, Error: err.Error()}
		}
		out.Top = append(out.Top, st)
	}
	return out
}
