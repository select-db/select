package engine

import (
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

	var created core.SQLDialect
	switch dbType {
	case "postgresql":
		created = postgresql.NewDialect()
	case "mysql":
		created = mysql.NewDialect()
	case "sqlite":
		created = sqlite.NewDialect()
	default:
		return nil
	}

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
