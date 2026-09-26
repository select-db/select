package query

import (
	"context"
	"sync"
)

type cancelEntry struct {
	cancel context.CancelFunc
}

var (
	cancelRegistry   = make(map[string]*cancelEntry)
	cancelRegistryMu sync.Mutex
)

// RegisterCancel stores cancel under key and cancels the run registered there
// before: a key is one file on one database, and running it again supersedes
// the earlier run. The returned func removes this registration only, so an
// earlier run finishing late cannot drop the cancel of the one that replaced it.
func RegisterCancel(key string, cancel context.CancelFunc) (unregister func()) {
	entry := &cancelEntry{cancel: cancel}

	cancelRegistryMu.Lock()
	previous := cancelRegistry[key]
	cancelRegistry[key] = entry
	cancelRegistryMu.Unlock()

	if previous != nil {
		previous.cancel()
	}

	return func() {
		cancelRegistryMu.Lock()
		if cancelRegistry[key] == entry {
			delete(cancelRegistry, key)
		}
		cancelRegistryMu.Unlock()
	}
}

// Cancel triggers the cancel func registered under key and removes it.
// No-op if key is not registered.
func Cancel(key string) {
	cancelRegistryMu.Lock()
	entry := cancelRegistry[key]
	delete(cancelRegistry, key)
	cancelRegistryMu.Unlock()
	if entry != nil {
		entry.cancel()
	}
}
