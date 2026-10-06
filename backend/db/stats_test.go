package db

import (
	"context"
	"testing"
)

func TestReadingIsKeptWithinItsTimeAndSharedByKey(t *testing.T) {
	reads := 0
	read := func(context.Context) int {
		reads++
		return reads
	}

	first := reading(context.Background(), "test-key", read)
	again := reading(context.Background(), "test-key", read)

	if first != 1 || again != 1 || reads != 1 {
		t.Fatalf("a second ask within the ttl must not read: %d %d after %d reads", first, again, reads)
	}
	if other := reading(context.Background(), "other-key", read); other != 2 {
		t.Fatalf("another key reads for itself: %d", other)
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
