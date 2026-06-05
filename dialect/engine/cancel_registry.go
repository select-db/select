package engine

import (
	"context"
	"sync"
)

var (
	cancelRegistry   = make(map[string]context.CancelFunc)
	cancelRegistryMu sync.Mutex
)

// RegisterCancel stores cancel under key, replacing any existing entry.
func RegisterCancel(key string, cancel context.CancelFunc) {
	cancelRegistryMu.Lock()
	cancelRegistry[key] = cancel
	cancelRegistryMu.Unlock()
}

// Cancel triggers the cancel func registered under key and removes it.
// No-op if key is not registered.
func Cancel(key string) {
	cancelRegistryMu.Lock()
	cancel := cancelRegistry[key]
	delete(cancelRegistry, key)
	cancelRegistryMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// UnregisterCancel removes the entry for key without calling the cancel func.
// Used in cleanup defers after the query finishes normally.
func UnregisterCancel(key string) {
	cancelRegistryMu.Lock()
	delete(cancelRegistry, key)
	cancelRegistryMu.Unlock()
}
