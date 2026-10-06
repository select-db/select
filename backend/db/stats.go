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

// StatsReport is the "db" section of /debug/stats.
type StatsReport struct {
	Pool     *PoolStats     `json:"pool,omitempty"`
	Postgres *PostgresStats `json:"postgres,omitempty"`
}

// statsTTL is how long the database's own counters are reused: the page that reads
// them asks every few seconds, and they need one query, not one per ask.
const statsTTL = 15 * time.Second

var postgresStats cache[PostgresStats]

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
	pg := postgresStats.get(time.Now(), statsTTL, readPostgresStats)
	report.Postgres = &pg
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
