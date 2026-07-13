// Package e2e drives the real HTTP API + sync path against Postgres and asserts
// that each operation emits the audit event its catalog spec declares. Specs
// whose emit site is not wired yet fail here by design — the suite is the
// executable checklist for wiring them. Tests must not run in parallel (the
// audit logger and db package are process-global, rebound per test).
package e2e

import (
	"testing"

	"backend/test/support"
)

func TestMain(m *testing.M) { support.Run(m) }
