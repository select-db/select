package utils

import (
	"sync"
	"time"

	"github.com/selectDb/toolkit/debounce"

	"selectDb/internal/desktop"
)

// DebouncedEventsEmitter holds one debouncer per event name.
type DebouncedEventsEmitter struct {
	debouncers map[string]*debounce.Debouncer
	mu         sync.RWMutex
}

var (
	globalEmitter     *DebouncedEventsEmitter
	globalEmitterOnce sync.Once
)

func GetDebouncedEventsEmitter() *DebouncedEventsEmitter {
	globalEmitterOnce.Do(func() {
		globalEmitter = &DebouncedEventsEmitter{
			debouncers: make(map[string]*debounce.Debouncer),
		}
	})
	return globalEmitter
}

// Emit debounces by event name and captures the payload now. Use EmitFunc when
// building the payload is expensive.
//
// A Debouncer keeps the window it was created with, so timeout applies only to
// the first call for an event name.
func (e *DebouncedEventsEmitter) Emit(eventName string, timeout time.Duration, payload ...interface{}) {
	e.mu.Lock()

	debouncer, exists := e.debouncers[eventName]
	if !exists {
		newDebouncer := debounce.NewDebounce(timeout, func() {
			desktop.Emit(eventName, payload...)
		})
		e.debouncers[eventName] = &newDebouncer
		debouncer = &newDebouncer
	} else {
		debouncer.UpdateDebounceCallback(func() {
			desktop.Emit(eventName, payload...)
		})
	}

	e.mu.Unlock()

	debouncer.Debounce()
}

// EmitFunc builds the payload when the event fires, not on every call. A burst
// keeps only the last payload, and the pending callback keeps whatever it
// captured alive until the next event replaces it.
func (e *DebouncedEventsEmitter) EmitFunc(eventName string, timeout time.Duration, buildPayload func() []interface{}) {
	e.mu.Lock()

	callback := func() { desktop.Emit(eventName, buildPayload()...) }

	debouncer, exists := e.debouncers[eventName]
	if !exists {
		newDebouncer := debounce.NewDebounce(timeout, callback)
		e.debouncers[eventName] = &newDebouncer
		debouncer = &newDebouncer
	} else {
		debouncer.UpdateDebounceCallback(callback)
	}

	e.mu.Unlock()

	debouncer.Debounce()
}

// DebouncedEventsEmitFunc is EmitFunc on the global emitter.
func DebouncedEventsEmitFunc(eventName string, timeout time.Duration, buildPayload func() []interface{}) {
	GetDebouncedEventsEmitter().EmitFunc(eventName, timeout, buildPayload)
}

// DebouncedEventsEmit is Emit on the global emitter.
func DebouncedEventsEmit(eventName string, timeout time.Duration, payload ...interface{}) {
	GetDebouncedEventsEmitter().Emit(eventName, timeout, payload...)
}
