package results

import (
	"time"

	"github.com/selectDb/toolkit/cache"
)

// resultCache is a process-wide store for query results, keyed by the
// caller-supplied key (the app uses db:<id>:file:<id>). App-only: the
// remote backend never caches; it streams rows.
//
// Entries are *StreamingResult so readers can observe progress while
// the writer goroutine still appends. Buffered query.Result returns from
// query.Execute are not cached; only query.Stream() populates this cache.
var resultCache = cache.New(cache.Options{TTL: 20 * time.Minute, MaxEntries: 10})

// Get looks up a *StreamingResult for key.
func Get(key string) (*StreamingResult, bool) {
	v, ok := resultCache.Get(key)
	if !ok {
		return nil, false
	}
	return v.(*StreamingResult), true
}

// Set stores a *StreamingResult under key.
func Set(key string, result *StreamingResult) {
	resultCache.Set(key, result)
}

// Delete removes the entry for key. No-op if absent.
func Delete(key string) {
	resultCache.Delete(key)
}

// Clear drops all entries.
func Clear() {
	resultCache.DeleteFunc(func(string) bool { return true })
}
