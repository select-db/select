package audit

import "context"

// Default is the process-wide logger, set once at startup by main. It is
// nil-safe: every method on *Logger tolerates a nil receiver, so emit sites can
// call the package helpers unconditionally even when logging is not configured
// (e.g. in tests).
var Default *Logger

// SetDefault installs the process-wide logger.
func SetDefault(l *Logger) { Default = l }

// Log emits on the async lane via the default logger.
func Log(e *Event) { Default.Log(e) }

// LogOutbox emits on the durable outbox lane via the default logger.
func LogOutbox(ctx context.Context, e *Event) error { return Default.LogOutbox(ctx, e) }
