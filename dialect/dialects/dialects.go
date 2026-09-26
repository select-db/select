// Package dialects is the registry of SQL dialects: the built-in postgresql,
// mysql and sqlite, and any a program registers.
package dialects

import (
	"maps"
	"slices"
	"sync"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/mysql"
	"github.com/selectDb/dialect/postgresql"
	"github.com/selectDb/dialect/sqlite"
)

var (
	registry   = make(map[string]core.SQLDialect)
	registryMu sync.RWMutex
)

// builtins is the one list of the dialects we ship. The tests that must
// hold for all of them range over it, so a fourth is enrolled by being added
// here rather than by someone remembering to widen each test.
var builtins = map[string]func() core.SQLDialect{
	"postgresql": func() core.SQLDialect { return postgresql.NewDialect() },
	"mysql":      func() core.SQLDialect { return mysql.NewDialect() },
	"sqlite":     func() core.SQLDialect { return sqlite.NewDialect() },
}

// Builtin names them in a stable order.
func Builtin() []string {
	return slices.Sorted(maps.Keys(builtins))
}

// Register stores a custom dialect under dbType. Optional: the three built-in
// types (postgresql, mysql, sqlite) are created lazily by Get.
func Register(dbType string, dialect core.SQLDialect) {
	registryMu.Lock()
	registry[dbType] = dialect
	registryMu.Unlock()
}

// Get returns the dialect for dbType, or nil for an unknown type. Built-in
// types are created on first call and cached; custom types are registered.
func Get(dbType string) core.SQLDialect {
	registryMu.RLock()
	dialect := registry[dbType]
	registryMu.RUnlock()
	if dialect != nil {
		return dialect
	}

	newDialect, builtin := builtins[dbType]
	if !builtin {
		return nil
	}
	created := newDialect()

	registryMu.Lock()
	// Re-check under write lock; another goroutine may have raced us.
	if existing := registry[dbType]; existing != nil {
		registryMu.Unlock()
		return existing
	}
	registry[dbType] = created
	registryMu.Unlock()
	return created
}
