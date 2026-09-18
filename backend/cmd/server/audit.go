package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"backend/db"
	"backend/internal/audit"
	"backend/internal/kms"
)

// startAuditLogger builds the unified audit logger, starts its background lanes,
// installs it as the package default, and provisions/validates partition
// maintenance. Returns the logger so the caller can Stop it on shutdown.
func startAuditLogger() *audit.Logger {
	// Async writer + outbox drainer run until Stop on shutdown. Partitions are
	// managed in-DB by pg_partman + pg_cron; without them, the logger sweeps rows
	// older than AUDIT_RETENTION_DAYS (default 365; 0 = keep forever) instead.
	logger := audit.New(db.GetDB(), audit.Options{RetentionDays: auditRetentionDays()})
	logger.Start()
	audit.SetDefault(logger)

	// pg_cron's scheduler lives in the cluster's cron DB, not the app DB, so the
	// maintenance job can't be a migration. No-op if POSTGRES_AUDIT_CRON_DSN is
	// unset, then it's provisioned out of band (see the on-prem runbook).
	if cronDSN, _ := kms.Secret("POSTGRES_AUDIT_CRON_DSN"); cronDSN != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		if err := audit.EnsureMaintenanceSchedule(ctx, db.GetDB(), cronDSN, os.Getenv("AUDIT_CRON_SCHEDULE")); err != nil {
			log.Printf("WARNING: audit: %v", err)
		}
		cancel()
	}

	// Report on partition maintenance: warns only when pg_partman is present but
	// misconfigured. No partman is a supported mode (in-app retention).
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := audit.Preflight(ctx, db.GetDB()); err != nil {
		log.Printf("WARNING: %v", err)
	}
	cancel()

	return logger
}

// stopAuditLogger drains buffered events after the HTTP server has stopped
// accepting requests, so no Log call races the channel close.
func stopAuditLogger(logger *audit.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := logger.Stop(ctx); err != nil {
		log.Printf("audit: shutdown drain: %v", err)
	}
}

// auditRetentionDays reads AUDIT_RETENTION_DAYS (used only when pg_partman is
// absent); defaults to one year, matching partman's retention.
func auditRetentionDays() int {
	if v := os.Getenv("AUDIT_RETENTION_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
		log.Printf("WARNING: invalid AUDIT_RETENTION_DAYS=%q, using default", v)
	}
	return 365
}
