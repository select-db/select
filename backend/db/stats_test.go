package db

import (
	"testing"
	"time"
)

func TestCacheFetchesOnceWithinItsTime(t *testing.T) {
	var c cache[int]
	fetches := 0
	fetch := func() int {
		fetches++
		return fetches
	}
	start := time.Now()

	if got := c.get(start, 15*time.Second, fetch); got != 1 {
		t.Fatalf("first read: %d", got)
	}
	if got := c.get(start.Add(14*time.Second), 15*time.Second, fetch); got != 1 || fetches != 1 {
		t.Fatalf("a read within the ttl must not fetch: %d after %d fetches", got, fetches)
	}
	if got := c.get(start.Add(15*time.Second), 15*time.Second, fetch); got != 2 {
		t.Fatalf("a read after the ttl fetches again: %d", got)
	}
}

func TestStatsWithoutAConnectionIsEmptyNotAPanic(t *testing.T) {
	saved := conn
	conn = nil
	defer func() { conn = saved }()
	if report := Stats().(StatsReport); report.Pool != nil || report.Postgres != nil {
		t.Fatalf("%+v", report)
	}
}
