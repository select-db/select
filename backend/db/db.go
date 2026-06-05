package db

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	"backend/db/generated"

	_ "github.com/lib/pq"
)

var (
	conn    *sql.DB
	Queries *generated.Queries
)

func Init() error {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		return fmt.Errorf("DB_DSN is not set")
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
