package toolkit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

var (
	statsMu sync.Mutex
	stats   = map[string]func() any{}
)

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
