package db

import (
	"database/sql"
	"fmt"
	"time"

	"backend/db/generated"
	"backend/internal/kms"

	_ "github.com/lib/pq"
)

var (
	conn    *sql.DB
	Queries *generated.Queries
)

func Init() error {
	// Dev: DB_DSN env. Prod: same name as a path in the OVH KMS secret store.
	dsn, err := kms.Secret("DB_DSN")
	if err != nil {
		return err
	}

	c, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("failed to open db: %w", err)
	}

	if err := c.Ping(); err != nil {
		return fmt.Errorf("db ping failed: %w", err)
	}

	c.SetMaxOpenConns(5)
	c.SetMaxIdleConns(2)
	c.SetConnMaxIdleTime(5 * time.Minute)
	c.SetConnMaxLifetime(1 * time.Hour)
	SetDB(c)

	return nil
}

func GetDB() *sql.DB {
	return conn
}

// SetDB replaces the global connection and queries.
func SetDB(c *sql.DB) {
	conn = c
	Queries = generated.New(c)
}

func Ping() error {
	return conn.Ping()
}
