package audit

import (
	"testing"
	"time"
)

func TestMonthlyPartitionName(t *testing.T) {
	got := monthlyPartitionName(DomainQuery, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if got != "event_query_2026_06" {
		t.Fatalf("name = %q, want event_query_2026_06", got)
	}
}

func TestPartitionMonthEnd(t *testing.T) {
	parent := parentName(DomainIAM) // event_iam

	end, ok := partitionMonthEnd(parent, "event_iam_2026_06")
	if !ok {
		t.Fatalf("expected a parseable monthly partition")
	}
	want := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if !end.Equal(want) {
		t.Fatalf("end = %v, want %v", end, want)
	}

	// December must roll into the next year.
	end, ok = partitionMonthEnd(parent, "event_iam_2026_12")
	if !ok || !end.Equal(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("december rollover wrong: %v ok=%v", end, ok)
	}

	// The DEFAULT partition and malformed names must never be treated as dated.
	for _, name := range []string{"event_iam_default", "event_iam", "event_iam_2026", "event_iam_2026_13"} {
		if _, ok := partitionMonthEnd(parent, name); ok {
			t.Fatalf("expected %q to be unparseable", name)
		}
	}
}

func TestRetentionFor(t *testing.T) {
	if got := retentionFor(DomainQuery); got != defaultRetention {
		t.Fatalf("default retention = %v, want %v", got, defaultRetention)
	}
}
