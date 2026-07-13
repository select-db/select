# package e2e — audit end-to-end harness

Shared harness (package `e2e`) for tests that drive the real HTTP API + sync
path against Postgres and assert each operation emits the audit event its
catalog spec declares (`backend/internal/audit/catalog.go`).

## Where the tests live

Audit tests are **co-located with the code that emits the event**, one
`audit_test.go` per emitting package, in an external `_test` package that
imports this harness:

    internal/syncer/permission/audit_test.go   // package permission_test
    internal/syncer/group/audit_test.go
    internal/datasource/audit_test.go
    …

Each such file starts with:

    func TestMain(m *testing.M) { e2e.Run(m) }

## Convention

- `..._IsWired` asserts a spec that already has an emit site — must stay **green**.
- `..._NotWiredYet` asserts a spec still to be wired — **red on purpose**, turns
  green when the site lands. The suite is the executable checklist for deploying
  the audit log everywhere.

## Running

One shared Postgres for the whole run (fast: one boot + one migrated template
reused across every package):

    TEST_PG_HOST=localhost TEST_PG_PASSWORD=postgres go test ./...

Without `TEST_PG_HOST`, each package boots its own embedded Postgres (zero setup,
but ~3.3s boot per package). `initdb` refuses to run as root, so run as a
non-root user (CI runners already are).

Tests must not call `t.Parallel()`: the audit logger and the `db` package are
process-global singletons rebound per test.

## Not covered

`auth.login` / `auth.login_failed` fire inside the GitHub device-code exchange,
which the harness deliberately does not drive (tokens are crafted directly via
`e2e.NewAccount`). They are left as explicit red TODOs until a login seam exists.
