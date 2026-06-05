package db

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"selectDb/internal/server"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

var (
	DB *sql.DB
	mu sync.RWMutex
)

// Init initializes the global DB for the given server domain.
// If domain is empty, the global DB is left nil (e.g. dev with no server selected).
func Init(domain string) error {
	mu.Lock()
	defer mu.Unlock()
	return initForDomain(domain)
}

func initForDomain(domain string) error {
	if domain == "" {
		DB = nil
		return nil
	}
	dbPath, err := dbPathForDomain(domain)
	if err != nil {
		return err
	}
	serverDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(serverDir, 0o700); err != nil {
		return fmt.Errorf("create server dir: %w", err)
	}
	if _, err := os.Stat(dbPath); errors.Is(err, os.ErrNotExist) {
		f, err := os.OpenFile(dbPath, os.O_RDWR|os.O_CREATE, 0o600)
		if err != nil {
			return fmt.Errorf("create db file: %w", err)
		}
		_ = f.Close()
	} else if err == nil {
		if err := os.Chmod(dbPath, 0o600); err != nil {
			return fmt.Errorf("chmod db file: %w", err)
		}
	}
	if err := runMigrations(dbPath); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	conn, err := sql.Open("sqlite3-mw", dbPath+"?_pragma=busy_timeout(2000)")
	if err != nil {
		return fmt.Errorf("open DB: %w", err)
	}
	conn.SetMaxOpenConns(2)
	if DB != nil {
		_ = DB.Close()
	}
	DB = conn
	return nil
}

// SwitchToServer closes the current DB and opens the DB for the given domain.
func SwitchToServer(domain string) error {
	mu.Lock()
	defer mu.Unlock()
	return initForDomain(domain)
}

// GetDB returns the global DB instance. May be nil if no server is selected.
func GetDB() *sql.DB {
	mu.RLock()
	defer mu.RUnlock()
	return DB
}

// RunMigrationsAt runs migrations at the given db path (used when creating a new server).
func RunMigrationsAt(dbPath string) error {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create server dir: %w", err)
	}
	if _, err := os.Stat(dbPath); errors.Is(err, os.ErrNotExist) {
		f, err := os.OpenFile(dbPath, os.O_RDWR|os.O_CREATE, 0o600)
		if err != nil {
			return fmt.Errorf("create db file: %w", err)
		}
		_ = f.Close()
	}
	return runMigrations(dbPath)
}

func dbPathForDomain(domain string) (string, error) {
	return server.ServerDBPath(domain)
}

func runMigrations(dbPath string) error {
	plainDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("open plain DB for migrations: %w", err)
	}
	defer func() { _ = plainDB.Close() }()

	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}
	return goose.Up(plainDB, "migrations")
}
