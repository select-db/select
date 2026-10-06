package db

import (
	"context"
	"sync"
	"time"
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
	LongestWaitMs int64  `json:"longestWaitMs"`
	Error         string `json:"error,omitempty"`
}

// TableStats is a table, among the largest of the database. DeadTuples against
// LiveTuples is the sign of bloat: autovacuum is behind when it grows.
type TableStats struct {
	Name           string `json:"name"`
	Bytes          int64  `json:"bytes"` // with its indexes and its toast
	LiveTuples     int64  `json:"liveTuples"`
	DeadTuples     int64  `json:"deadTuples"`
	SeqScans       int64  `json:"seqScans"`
	IdxScans       int64  `json:"idxScans"`
	LastAutovacuum int64  `json:"lastAutovacuum,omitempty"` // unix seconds
}

// ReplicationStats is the standbys of this database, if it has any.
type ReplicationStats struct {
	Replicas int    `json:"replicas"`
	MaxLagMs int64  `json:"maxLagMs"`
	Error    string `json:"error,omitempty"`
}

// Statement is a normalised query of pg_stat_statements: no values, the placeholders.
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
	Tables      []TableStats      `json:"tables,omitempty"`
	Replication *ReplicationStats `json:"replication,omitempty"`
	Statements  *StatementsStats  `json:"statements,omitempty"`
}

// statsTTL is how long the database's own counters are reused: the page that reads
// them asks every few seconds, and they need one query, not one per ask.
const statsTTL = 15 * time.Second

// slowStatsTTL is for what is read from larger views: the tables, the locks, the
// standbys and the statements change slowly, and cost a little more than a counter.
const slowStatsTTL = time.Minute

var (
	postgresStats    cache[PostgresStats]
	lockStats        cache[LockStats]
	tableStats       cache[[]TableStats]
	replicationStats cache[ReplicationStats]
	statementStats   cache[StatementsStats]
)

// Stats is read only when /debug/stats is, so the pool is counted and the database
// asked (one query on a catalog view, at most every statsTTL) only while somebody
// is looking.
func Stats() any {
	if conn == nil {
		return StatsReport{}
	}
	pool := conn.Stats()
	report := StatsReport{Pool: &PoolStats{
		Max:    pool.MaxOpenConnections,
		Open:   pool.OpenConnections,
		InUse:  pool.InUse,
		Idle:   pool.Idle,
		Waits:  pool.WaitCount,
		WaitMs: pool.WaitDuration.Milliseconds(),
	}}
	now := time.Now()
	pg := postgresStats.get(now, statsTTL, readPostgresStats)
	report.Postgres = &pg
	locks := lockStats.get(now, statsTTL, readLockStats)
	report.Locks = &locks
	report.Tables = tableStats.get(now, slowStatsTTL, readTableStats)
	replication := replicationStats.get(now, slowStatsTTL, readReplicationStats)
	report.Replication = &replication
	statements := statementStats.get(now, slowStatsTTL, readStatementStats)
	report.Statements = &statements
	return report
}

func readPostgresStats() PostgresStats {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
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

// each read has its own two seconds and its own connection from the pool, one at a time
func queryContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 2*time.Second)
}

func readLockStats() LockStats {
	ctx, cancel := queryContext()
	defer cancel()
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

func readTableStats() []TableStats {
	ctx, cancel := queryContext()
	defer cancel()
	rows, err := conn.QueryContext(ctx, `
		SELECT relname, pg_total_relation_size(relid), n_live_tup, n_dead_tup,
		       coalesce(seq_scan, 0), coalesce(idx_scan, 0),
		       coalesce(extract(epoch FROM last_autovacuum)::bigint, 0)
		FROM pg_stat_user_tables ORDER BY pg_total_relation_size(relid) DESC LIMIT 8`)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()
	var tables []TableStats
	for rows.Next() {
		var t TableStats
		if rows.Scan(&t.Name, &t.Bytes, &t.LiveTuples, &t.DeadTuples, &t.SeqScans, &t.IdxScans, &t.LastAutovacuum) == nil {
			tables = append(tables, t)
		}
	}
	return tables
}

func readReplicationStats() ReplicationStats {
	ctx, cancel := queryContext()
	defer cancel()
	var r ReplicationStats
	err := conn.QueryRowContext(ctx, `
		SELECT count(*), coalesce(max(extract(epoch FROM replay_lag) * 1000), 0)::bigint
		FROM pg_stat_replication`).Scan(&r.Replicas, &r.MaxLagMs)
	if err != nil {
		return ReplicationStats{Error: err.Error()}
	}
	return r
}

// readStatementStats reads pg_stat_statements when it is installed in this database,
// and says so when it is not. The queries it holds are normalised, with no values.
func readStatementStats() StatementsStats {
	ctx, cancel := queryContext()
	defer cancel()
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
		if rows.Scan(&st.Query, &st.Calls, &st.TotalMs, &st.MeanMs, &st.Rows) == nil {
			out.Top = append(out.Top, st)
		}
	}
	return out
}

// cache keeps one value for a time. Concurrent readers share the one fetch.
type cache[T any] struct {
	mu    sync.Mutex
	at    time.Time
	value T
	has   bool
}

func (c *cache[T]) get(now time.Time, ttl time.Duration, fetch func() T) T {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.has || now.Sub(c.at) >= ttl {
		c.value, c.at, c.has = fetch(), now, true
	}
	return c.value
}
