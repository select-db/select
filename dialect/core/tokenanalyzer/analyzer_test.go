package tokenanalyzer_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/selectDb/dialect/core/tokenanalyzer"
)

// resolveTestAnalyzer finds the venv python + main.py relative to this test file.
func resolveTestAnalyzer(t *testing.T) (string, string) {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}

	pyDir := filepath.Join(filepath.Dir(thisFile), "python")
	script := filepath.Join(pyDir, "main.py")

	venvBin := "bin"
	if runtime.GOOS == "windows" {
		venvBin = "Scripts"
	}
	pyPath := filepath.Join(pyDir, ".venv", venvBin, "python3")

	if _, err := os.Stat(pyPath); err != nil {
		t.Skipf("python venv not found at %s", pyPath)
	}
	if _, err := os.Stat(script); err != nil {
		t.Skipf("main.py not found at %s", script)
	}
	return pyPath, script
}

func TestCall_Ping(t *testing.T) {
	pyPath, script := resolveTestAnalyzer(t)
	a := tokenanalyzer.NewAnalyzer(pyPath, script)
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
	pyPath, script := resolveTestAnalyzer(t)
	a := tokenanalyzer.NewAnalyzer(pyPath, script)
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
	pyPath, script := resolveTestAnalyzer(t)
	a := tokenanalyzer.NewAnalyzer(pyPath, script)
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
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		b.Fatal("cannot determine test file path")
	}
	pyDir := filepath.Join(filepath.Dir(thisFile), "python")
	script := filepath.Join(pyDir, "main.py")
	venvBin := "bin"
	if runtime.GOOS == "windows" {
		venvBin = "Scripts"
	}
	pyPath := filepath.Join(pyDir, ".venv", venvBin, "python3")
	if _, err := os.Stat(pyPath); err != nil {
		b.Skipf("python venv not found at %s", pyPath)
	}

	a := tokenanalyzer.NewAnalyzer(pyPath, script)
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
