package toolkit

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestStatsAreReadOnlyWhenAskedAndOneBadSectionDoesNotSpoilTheRest(t *testing.T) {
	statsMu.Lock()
	stats = map[string]func() any{}
	statsMu.Unlock()
	defer func() {
		statsMu.Lock()
		stats = map[string]func() any{"runtime": runtimeStats}
		statsMu.Unlock()
	}()

	reads := 0
	RegisterStats("pool", func() any {
		reads++
		return map[string]int{"inUse": 2}
	})
	RegisterStats("broken", func() any { panic("no database") })
	if reads != 0 {
		t.Fatal("a section must not run when it is registered")
	}

	recorder := httptest.NewRecorder()
	serveStats(recorder, httptest.NewRequest("GET", "/debug/stats", nil))

	var got map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %v: %s", err, recorder.Body.String())
	}
	if reads != 1 || string(got["pool"]) != `{"inUse":2}` {
		t.Fatalf("pool read %d times: %s", reads, got["pool"])
	}
	if string(got["broken"]) != `{"error":"no database"}` {
		t.Fatalf("broken: %s", got["broken"])
	}
}

func TestEveryProcessReportsItsRuntime(t *testing.T) {
	recorder := httptest.NewRecorder()
	serveStats(recorder, httptest.NewRequest("GET", "/debug/stats", nil))

	var got struct {
		Runtime RuntimeStats `json:"runtime"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("%v: %s", err, recorder.Body.String())
	}
	if got.Runtime.Goroutines < 1 || got.Runtime.NumCPU < 1 || got.Runtime.SysBytes == 0 || got.Runtime.HeapInuseBytes == 0 {
		t.Fatalf("%+v", got.Runtime)
	}
}
