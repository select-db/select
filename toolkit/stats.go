package toolkit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"
)

var (
	statsMu sync.Mutex
	stats   = map[string]func() any{"runtime": runtimeStats}
	started = time.Now()
)

// RuntimeStats is the "runtime" section, in every process that serves /debug/stats.
type RuntimeStats struct {
	Goroutines     int     `json:"goroutines"`
	HeapAllocBytes uint64  `json:"heapAllocBytes"` // live and not yet swept
	HeapInuseBytes uint64  `json:"heapInuseBytes"`
	SysBytes       uint64  `json:"sysBytes"` // all the memory the Go runtime has from the system
	GCCount        uint32  `json:"gcCount"`
	GCPauseTotalMs float64 `json:"gcPauseTotalMs"`
	NumCPU         int     `json:"numCPU"`
	GoMaxProcs     int     `json:"goMaxProcs"`
	UptimeSeconds  int64   `json:"uptimeSeconds"`
}

// runtimeStats reads the runtime's counters. ReadMemStats stops the world for a few
// microseconds, once per read of /debug/stats and never otherwise.
func runtimeStats() any {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return RuntimeStats{
		Goroutines:     runtime.NumGoroutine(),
		HeapAllocBytes: m.HeapAlloc,
		HeapInuseBytes: m.HeapInuse,
		SysBytes:       m.Sys,
		GCCount:        m.NumGC,
		GCPauseTotalMs: float64(m.PauseTotalNs) / 1e6,
		NumCPU:         runtime.NumCPU(),
		GoMaxProcs:     runtime.GOMAXPROCS(0),
		UptimeSeconds:  int64(time.Since(started).Seconds()),
	}
}

// RegisterStats adds a named section to /debug/stats, on the pprof server.
//
// fn runs only when somebody reads /debug/stats, never otherwise, so a section
// costs nothing while nobody looks: it can read counters the program keeps anyway
// or run a cheap query, and should cache what is not free for a few seconds.
// The section is the JSON of what fn returns.
func RegisterStats(name string, fn func() any) {
	statsMu.Lock()
	defer statsMu.Unlock()
	stats[name] = fn
}

// serveStats answers with every section, a failing one as {"error": ...}.
func serveStats(w http.ResponseWriter, _ *http.Request) {
	statsMu.Lock()
	sections := make(map[string]func() any, len(stats))
	for name, fn := range stats {
		sections[name] = fn
	}
	statsMu.Unlock()

	out := make(map[string]any, len(sections))
	for name, fn := range sections {
		out[name] = readSection(fn)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func readSection(fn func() any) (section any) {
	defer func() {
		if recovered := recover(); recovered != nil {
			section = map[string]string{"error": fmt.Sprint(recovered)}
		}
	}()
	return fn()
}
