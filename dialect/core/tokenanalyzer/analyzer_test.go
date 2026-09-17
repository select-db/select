package tokenanalyzer_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/selectDb/dialect/core/testutil"
)

func TestCall_Ping(t *testing.T) {
	a := testutil.NewTestAnalyzer(t)
	defer a.Close()

	raw, err := a.Call(map[string]any{"action": "ping"})
	if err != nil {
		t.Fatalf("Call ping: %v", err)
	}

	var resp struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal ping response: %v", err)
	}
	if !resp.OK {
		t.Errorf("expected ok=true, got %s", string(raw))
	}
}

func TestCallWithTimeout_Ping(t *testing.T) {
	a := testutil.NewTestAnalyzer(t)
	defer a.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	raw, err := a.CallWithTimeout(ctx, map[string]any{"action": "ping"})
	if err != nil {
		t.Fatalf("CallWithTimeout ping: %v", err)
	}

	var resp struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.OK {
		t.Errorf("expected ok=true, got %s", string(raw))
	}
}

func TestCallWithTimeout_Expired(t *testing.T) {
	a := testutil.NewTestAnalyzer(t)
	defer a.Close()

	// Use an already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := a.CallWithTimeout(ctx, map[string]any{"action": "ping"})
	if err == nil {
		t.Fatal("expected error from expired context")
	}
	if !os.IsTimeout(err) && err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func BenchmarkCall_Ping(b *testing.B) {
	a := testutil.NewTestAnalyzer(b)
	defer a.Close()

	// Warm up
	if _, err := a.Call(map[string]any{"action": "ping"}); err != nil {
		b.Fatalf("warm-up: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := a.Call(map[string]any{"action": "ping"}); err != nil {
			b.Fatalf("iteration %d: %v", i, err)
		}
	}
}

// TestCall_SurfacesAnalyzerError pins that a failure the subprocess reports in
// band reaches the caller. It answers with an error key rather than a broken
// pipe, so a caller decoding into its own type reads a failure as zero values.
func TestCall_SurfacesAnalyzerError(t *testing.T) {
	a := testutil.NewTestAnalyzer(t)
	defer a.Close()

	tests := []struct {
		name string
		req  map[string]any
		want []string
	}{
		{
			name: "unknown action",
			req:  map[string]any{"action": "no_such_action"},
			want: []string{"analyzer:", "Unknown action", "no_such_action"},
		},
		{
			// An exception raised mid-dispatch, which is what a bug in the
			// analyzer, or a caller sending the wrong shape, looks like here.
			name: "exception while handling a known action",
			req:  map[string]any{"action": "lint", "sql": "SELECT 1", "dialect": "postgresql", "schema": "not a dict"},
			want: []string{"analyzer:", "Traceback"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := a.Call(tt.req)
			if err == nil {
				t.Fatalf("expected an error, got response %s", string(raw))
			}
			for _, want := range tt.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error = %q, want it to contain %q", err, want)
				}
			}
		})
	}
}

// TestCall_KeepsServingAfterAnError pins that an error is reported without
// taking the subprocess down with it, since the caret moves on and the next
// keystroke asks again.
func TestCall_KeepsServingAfterAnError(t *testing.T) {
	a := testutil.NewTestAnalyzer(t)
	defer a.Close()

	if _, err := a.Call(map[string]any{"action": "no_such_action"}); err == nil {
		t.Fatal("expected an error from an unknown action")
	}

	if _, err := a.Call(map[string]any{"action": "ping"}); err != nil {
		t.Fatalf("ping after an error: %v", err)
	}
}

// TestAnalyze_LargeFile pins the read buffer. One response is one line, and
// bufio's 64 KiB default cap turned a few kilobytes of SQL into a read failure
// that lint and completion swallowed as "nothing here".
func TestAnalyze_LargeFile(t *testing.T) {
	a := testutil.NewTestAnalyzer(t)
	defer a.Close()

	var sql strings.Builder
	for i := 0; i < 600; i++ {
		fmt.Fprintf(&sql, "SELECT %d FROM no_such_table_%d;\n", i, i)
	}
	raw, err := a.Call(map[string]any{
		"action": "lint", "sql": sql.String(), "dialect": "postgresql",
		"schema": map[string]any{}, "default_schema": "public",
	})
	if err != nil {
		t.Fatalf("lint a large file: %v", err)
	}
	if len(raw) <= 64<<10 {
		t.Fatalf("response is %d bytes, under the old 64 KiB cap: the test no longer covers it", len(raw))
	}
}
