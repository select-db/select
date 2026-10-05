package query

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type valueSink struct {
	countingSink
	err error
}

func (s *valueSink) OnError(err error) {
	s.err = err
	s.countingSink.OnError(err)
}

func streamValue(t *testing.T, statement string, max int64) *valueSink {
	t.Helper()
	db := newDBWithRows(t, 0)
	defer db.Close()
	sink := &valueSink{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	Stream(ctx, Conn{DB: db}, Datasource{ID: "x"}, statement, Options{MaxValueBytes: max}, sink)
	return sink
}

func TestStream_MaxValueBytesRefusesAWideValue(t *testing.T) {
	sink := streamValue(t, "SELECT 1 AS id, zeroblob(2048) AS payload", 1024)

	if !sink.errored || sink.rows != 0 {
		t.Fatalf("want an error and no row, got errored=%v rows=%d", sink.errored, sink.rows)
	}
	if !strings.Contains(sink.err.Error(), `"payload"`) || !strings.Contains(sink.err.Error(), "limit per value") {
		t.Fatalf("the error should name the column and the limit: %v", sink.err)
	}
}

func TestStream_MaxValueBytesLetsValuesUnderItThrough(t *testing.T) {
	sink := streamValue(t, "SELECT 1 AS id, zeroblob(512) AS payload, 'text' AS note", 1024)

	if sink.errored || sink.rows != 1 || !sink.done {
		t.Fatalf("want one clean row, got errored=%v rows=%d done=%v", sink.errored, sink.rows, sink.done)
	}
}

func TestStream_NoMaxValueBytesMeansNoLimit(t *testing.T) {
	sink := streamValue(t, "SELECT zeroblob(1048576) AS payload", 0)

	if sink.errored || sink.rows != 1 {
		t.Fatalf("the desktop app sets no limit, got errored=%v rows=%d", sink.errored, sink.rows)
	}
}

func TestStream_ReserveIsAskedAsRowsGetWider(t *testing.T) {
	db := newDBWithRows(t, 0)
	defer db.Close()
	var asked []int64
	sink := &valueSink{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	Stream(ctx, Conn{DB: db}, Datasource{ID: "x"},
		"WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM c LIMIT 4) SELECT zeroblob(x * 1000) AS b FROM c",
		Options{Reserve: func(n int64) error { asked = append(asked, n); return nil }}, sink)

	if sink.errored || sink.rows != 4 {
		t.Fatalf("want 4 clean rows, got errored=%v rows=%d", sink.errored, sink.rows)
	}
	want := []int64{5 * 1000, 5 * 2000, 5 * 3000, 5 * 4000}
	if len(asked) != len(want) {
		t.Fatalf("asked %v, want %v", asked, want)
	}
	for i := range want {
		if asked[i] != want[i] {
			t.Fatalf("asked %v, want %v", asked, want)
		}
	}
}

func TestStream_AReservationThatFailsEndsTheStatement(t *testing.T) {
	db := newDBWithRows(t, 0)
	defer db.Close()
	refused := errors.New("no room")
	sink := &valueSink{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	Stream(ctx, Conn{DB: db}, Datasource{ID: "x"}, "SELECT zeroblob(4096) AS b",
		Options{Reserve: func(int64) error { return refused }}, sink)

	if !errors.Is(sink.err, refused) || sink.rows != 0 {
		t.Fatalf("want the reservation's error and no row, got err=%v rows=%d", sink.err, sink.rows)
	}
}

func TestStream_NarrowRowsAskForTheirOwnSize(t *testing.T) {
	db := newDBWithRows(t, 200)
	defer db.Close()
	var widest int64
	sink := &countingSink{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	Stream(ctx, Conn{DB: db}, Datasource{ID: "x"}, "SELECT id FROM t ORDER BY id",
		Options{Reserve: func(n int64) error { widest = max(widest, n); return nil }}, sink)

	if widest != rowFootprint*8 {
		t.Fatalf("a row of one integer should ask for %d bytes, widest ask was %d", rowFootprint*8, widest)
	}
}
