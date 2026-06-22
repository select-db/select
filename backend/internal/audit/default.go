package audit

import "context"

// Default is the process-wide logger. nil-safe (every *Logger method tolerates a
// nil receiver), so emit sites can call these unconditionally, e.g. in tests.
var Default *Logger

func SetDefault(l *Logger) { Default = l }

func Log(e *Event) { Default.Log(e) }

func LogOutbox(ctx context.Context, e *Event) error { return Default.LogOutbox(ctx, e) }
