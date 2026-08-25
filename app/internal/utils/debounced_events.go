package utils

import (
	"sync"
	"time"

	"github.com/selectDb/toolkit/debounce"

	"selectDb/internal/desktop"
)

// DebouncedEventsEmitter manages debounced event emissions
type DebouncedEventsEmitter struct {
	debouncers map[string]*debounce.Debouncer
	mutex      sync.RWMutex
}

var (
	globalEmitter     *DebouncedEventsEmitter
	globalEmitterOnce sync.Once
)

// GetDebouncedEventsEmitter returns the singleton instance of DebouncedEventsEmitter
func GetDebouncedEventsEmitter() *DebouncedEventsEmitter {
	globalEmitterOnce.Do(func() {
		globalEmitter = &DebouncedEventsEmitter{
			debouncers: make(map[string]*debounce.Debouncer),
		}
	})
	return globalEmitter
}

// Emit emits an event with debouncing based on the event name
// Subsequent calls with the same eventName within the timeout period will be debounced
func (e *DebouncedEventsEmitter) Emit(eventName string, timeout time.Duration, data ...interface{}) {
	e.mutex.Lock()

	// Get or create debouncer for this event
	debouncer, exists := e.debouncers[eventName]
	if !exists {
		// Create new debouncer with callback
		newDebouncer := debounce.NewDebounce(timeout, func() {
			desktop.Emit(eventName, data...)
		})
		e.debouncers[eventName] = &newDebouncer
		debouncer = &newDebouncer
	} else {
		// Update callback with new data
		debouncer.UpdateDebounceCallback(func() {
			desktop.Emit(eventName, data...)
		})
	}

	e.mutex.Unlock()

	// Trigger debounce
	debouncer.Debounce()
}

// DebouncedEventsEmit is a convenience function that uses the global emitter
// It debounces desktop.Emit calls based on eventName
//
// Example usage:
//
//	utils.DebouncedEventsEmit("databaseAvailability", 500*time.Millisecond, map[string]interface{}{
//	    "databases": databases,
//	})
func DebouncedEventsEmit(eventName string, timeout time.Duration, data ...interface{}) {
	GetDebouncedEventsEmitter().Emit(eventName, timeout, data...)
}
