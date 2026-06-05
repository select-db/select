package engine

import (
	"time"

	"github.com/selectDb/toolkit/cache"
)

// resultCache is a process-wide store for query results, keyed by the
// caller-supplied key (the app uses db:<id>:file:<id>). App-only: the
// remote backend never caches; it streams rows.
//
// Entries are *StreamingResult so readers can observe progress while
// the writer goroutine still appends. Buffered Result returns from
// Execute are not cached; only Stream() populates this cache.
var resultCache = cache.New(cache.Options{TTL: 20 * time.Minute, MaxEntries: 10})

// GetStreamingResult looks up a *StreamingResult for key.
func GetStreamingResult(key string) (*StreamingResult, bool) {
	v, ok := resultCache.Get(key)
	if !ok {
		return nil, false
	}
	return v.(*StreamingResult), true
}

// SetStreamingResult stores a *StreamingResult under key.
func SetStreamingResult(key string, result *StreamingResult) {
	resultCache.Set(key, result)
}

// DeleteResult removes the entry for key. No-op if absent.
func DeleteResult(key string) {
	resultCache.Delete(key)
}

// ClearResultCache drops all entries.
func ClearResultCache() {
	resultCache.DeleteFunc(func(string) bool { return true })
}
