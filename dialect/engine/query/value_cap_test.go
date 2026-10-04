package query

import (
	"context"
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
