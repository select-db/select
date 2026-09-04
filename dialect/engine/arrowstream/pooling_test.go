package arrowstream

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
)

// rows encodes a labelled result set through a pooled Sink and reads it back
// through a pooled Stream, returning the decoded rows.
func roundTrip(t *testing.T, label string, n int) [][]any {
	t.Helper()
	var buf bytes.Buffer
	s := NewSink(&buf)
	s.SetColumnTypeHints([]string{"INT8", "TEXT"})
	if err := s.OnColumns([]string{"id", "payload"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		if err := s.OnRow([]any{int64(i), fmt.Sprintf("%s-row-%d", label, i)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.OnDone(int64(n), 0, 1); err != nil {
		t.Fatal(err)
	}
	s.Close()

	st, err := NewStream(io.NopCloser(bytes.NewReader(buf.Bytes())))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.Columns(); err != nil {
		t.Fatal(err)
	}
	var out [][]any
	for {
		row, ok, err := st.Next()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		out = append(out, row)
	}
	return out
}

// TestPooledEncoderDoesNotLeakAcrossQueries is the tenancy guarantee: a
// recycled encoder must not carry bytes or match history from the previous
// query into the next one. Each query uses a distinct, highly compressible
// label, so a stale window would surface as wrong bytes or a decode error.
func TestPooledEncoderDoesNotLeakAcrossQueries(t *testing.T) {
	labels := []string{
		strings.Repeat("tenant-alpha-secret", 40),
		strings.Repeat("tenant-bravo-secret", 40),
		strings.Repeat("tenant-charlie-secret", 40),
	}
	for pass := 0; pass < 4; pass++ {
		for _, label := range labels {
			rows := roundTrip(t, label, 1200)
			if len(rows) != 1200 {
				t.Fatalf("label %.20s: got %d rows, want 1200", label, len(rows))
			}
			for i, row := range rows {
				want := fmt.Sprintf("%s-row-%d", label, i)
				if got := row[1].(string); got != want {
					t.Fatalf("row %d: payload mismatch\n got %.80s\nwant %.80s", i, got, want)
				}
			}
			// Nothing from another tenant may appear anywhere in this result.
			for _, other := range labels {
				if other == label {
					continue
				}
				for _, row := range rows {
					if strings.Contains(row[1].(string), other[:20]) {
						t.Fatalf("cross-query contamination: found %.20s in a %.20s result", other, label)
					}
				}
			}
		}
	}
}

// TestPooledCodecsUnderConcurrency runs many streams at once. Run with -race:
// if a pooled encoder or decoder were ever handed to two streams at the same
// time, this fails or trips the detector.
func TestPooledCodecsUnderConcurrency(t *testing.T) {
	const workers = 32
	const perWorker = 8

	var wg sync.WaitGroup
	errs := make(chan error, workers*perWorker)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for k := 0; k < perWorker; k++ {
				label := fmt.Sprintf("w%d-k%d", w, k)
				var buf bytes.Buffer
				s := NewSink(&buf)
				s.SetColumnTypeHints([]string{"INT8", "TEXT"})
				_ = s.OnColumns([]string{"id", "payload"})
				for i := 0; i < 700; i++ {
					_ = s.OnRow([]any{int64(i), label + "-" + strings.Repeat("x", i%64)})
				}
				_ = s.OnDone(700, 0, 1)
				s.Close()

				st, err := NewStream(io.NopCloser(bytes.NewReader(buf.Bytes())))
				if err != nil {
					errs <- err
					return
				}
				if _, err := st.Columns(); err != nil {
					errs <- err
					_ = st.Close()
					return
				}
				count := 0
				for {
					row, ok, err := st.Next()
					if err != nil {
						errs <- err
						break
					}
					if !ok {
						break
					}
					if !strings.HasPrefix(row[1].(string), label+"-") {
						errs <- fmt.Errorf("%s: foreign payload %.40s", label, row[1].(string))
						break
					}
					count++
				}
				_ = st.Close()
				if count != 700 {
					errs <- fmt.Errorf("%s: got %d rows, want 700", label, count)
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

// TestSinkCloseIsIdempotent guards the worst pooling failure mode: a double
// Close returning the same encoder to the pool twice, after which two
// concurrent queries could be writing through one encoder.
func TestSinkCloseIsIdempotent(t *testing.T) {
	var buf bytes.Buffer
	s := NewSink(&buf)
	s.SetColumnTypeHints([]string{"INT8"})
	_ = s.OnColumns([]string{"id"})
	_ = s.OnRow([]any{int64(1)})
	_ = s.OnDone(1, 0, 1)

	before := s.zw
	if before == nil {
		t.Fatal("sink has no encoder")
	}
	s.Close()
	s.Close()
	s.Close()
	if s.zw != nil {
		t.Fatal("encoder reference retained after Close")
	}

	a := encoderPool.Get()
	b := encoderPool.Get()
	if a == b {
		t.Fatal("the same encoder was pooled twice")
	}
	encoderPool.Put(a)
	encoderPool.Put(b)
}

func TestStreamCloseIsIdempotent(t *testing.T) {
	var buf bytes.Buffer
	s := NewSink(&buf)
	s.SetColumnTypeHints([]string{"INT8"})
	_ = s.OnColumns([]string{"id"})
	_ = s.OnRow([]any{int64(1)})
	_ = s.OnDone(1, 0, 1)
	s.Close()

	st, err := NewStream(io.NopCloser(bytes.NewReader(buf.Bytes())))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	a := decoderPool.Get()
	b := decoderPool.Get()
	if a == b {
		t.Fatal("the same decoder was pooled twice")
	}
	decoderPool.Put(a)
	decoderPool.Put(b)
}

type failingWriter struct {
	after int
	n     int
}

func (f *failingWriter) Write(p []byte) (int, error) {
	f.n += len(p)
	if f.n > f.after {
		return 0, errors.New("client disconnected")
	}
	return len(p), nil
}

// TestPooledEncoderRecoversFromBrokenStream: a query whose client vanished
// mid-stream leaves the encoder holding a write error and partial state. It
// must come back from the pool clean, not poison the next tenant's result.
func TestPooledEncoderRecoversFromBrokenStream(t *testing.T) {
	for i := 0; i < 5; i++ {
		fw := &failingWriter{after: 256}
		s := NewSink(fw)
		s.SetColumnTypeHints([]string{"INT8", "TEXT"})
		_ = s.OnColumns([]string{"id", "payload"})
		for j := 0; j < 5000; j++ {
			_ = s.OnRow([]any{int64(j), strings.Repeat("aborted-tenant-data", 8)})
		}
		_ = s.OnDone(5000, 0, 1)
		s.Close()
	}

	rows := roundTrip(t, "healthy-tenant", 900)
	if len(rows) != 900 {
		t.Fatalf("got %d rows, want 900", len(rows))
	}
	for i, row := range rows {
		want := fmt.Sprintf("healthy-tenant-row-%d", i)
		got := row[1].(string)
		if got != want {
			t.Fatalf("row %d: got %q want %q", i, got, want)
		}
		if strings.Contains(got, "aborted-tenant-data") {
			t.Fatalf("row %d carries data from an aborted stream", i)
		}
	}
}
