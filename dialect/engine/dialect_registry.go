package engine

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
	dialectRegistry   = make(map[string]core.SQLDialect)
	dialectRegistryMu sync.RWMutex
)

// builtinDialects is the one list of the dialects we ship. The tests that must
// hold for all of them range over it, so a fourth is enrolled by being added
// here rather than by someone remembering to widen each test.
var builtinDialects = map[string]func() core.SQLDialect{
	"postgresql": func() core.SQLDialect { return postgresql.NewDialect() },
	"mysql":      func() core.SQLDialect { return mysql.NewDialect() },
	"sqlite":     func() core.SQLDialect { return sqlite.NewDialect() },
}

// BuiltinDialects names them in a stable order.
func BuiltinDialects() []string {
	return slices.Sorted(maps.Keys(builtinDialects))
}

// RegisterDialect stores a custom dialect under dbType. Optional: the three
// built-in types (postgresql, mysql, sqlite) are created lazily by GetDialect.
func RegisterDialect(dbType string, dialect core.SQLDialect) {
	dialectRegistryMu.Lock()
	dialectRegistry[dbType] = dialect
	dialectRegistryMu.Unlock()
}

// GetDialect returns the dialect for dbType. Built-in types are created on
// first call and cached; custom types must be pre-registered via RegisterDialect.
func GetDialect(dbType string) core.SQLDialect {
	dialectRegistryMu.RLock()
	dialect := dialectRegistry[dbType]
	dialectRegistryMu.RUnlock()
	if dialect != nil {
		return dialect
	}

	newDialect, builtin := builtinDialects[dbType]
	if !builtin {
		return nil
	}
	created := newDialect()

	dialectRegistryMu.Lock()
	// Re-check under write lock; another goroutine may have raced us.
	if existing := dialectRegistry[dbType]; existing != nil {
		dialectRegistryMu.Unlock()
		return existing
	}
	dialectRegistry[dbType] = created
	dialectRegistryMu.Unlock()
	return created
}
