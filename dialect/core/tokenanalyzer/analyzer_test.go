package tokenanalyzer_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/selectDb/dialect/core/testutil"
	"github.com/selectDb/dialect/core/tokenanalyzer"
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
	pythonPath, script, ok := tokenanalyzer.FindDevAnalyzer()
	if !ok {
		b.Skip("python venv not found; run `uv sync` in dialect/core/tokenanalyzer/python")
	}

	a := tokenanalyzer.NewAnalyzer(pythonPath, script)
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

// TestCall_SurfacesAnalyzerError covers the failure the subprocess reports in
// band. Every error it raises comes back as a response carrying an error key,
// and a caller that unmarshals such a response into its own type reads it as
// zero values, so completion showed nothing rather than reporting a failure.
func TestCall_SurfacesAnalyzerError(t *testing.T) {
	a := testutil.NewTestAnalyzer(t)
	defer a.Close()

	tests := []struct {
		name string
		req  map[string]any
		want string
	}{
		{
			name: "unknown action",
			req:  map[string]any{"action": "no_such_action"},
			want: "Unknown action",
		},
		{
			// An exception raised mid-dispatch, which is what a bug in the
			// analyzer, or a caller sending the wrong shape, looks like here.
			name: "exception while handling a known action",
			req:  map[string]any{"action": "lint", "sql": "SELECT 1", "dialect": "postgresql", "schema": "not a dict"},
			want: "analyzer:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := a.Call(tt.req)
			if err == nil {
				t.Fatalf("expected an error, got response %s", string(raw))
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to contain %q", err, tt.want)
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

	raw, err := a.Call(map[string]any{"action": "ping"})
	if err != nil {
		t.Fatalf("ping after an error: %v", err)
	}
	var resp struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil || !resp.OK {
		t.Errorf("ping after an error returned %s", string(raw))
	}
}
