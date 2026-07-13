# audit e2e suite

End-to-end tests that drive the real HTTP API + sync path against an embedded
Postgres and assert each operation emits the audit event its catalog spec
declares (`backend/internal/audit/catalog.go`).

## Convention

- A test named `..._IsWired` asserts a spec that already has an emit site — it
  must stay **green**.
- A test named `..._NotWiredYet` asserts a spec whose emit site is still to be
  wired — it is **red on purpose** and turns green when the site lands. The
  suite is the executable checklist for deploying the audit log everywhere.

## Running

`initdb` refuses to run as root, so run the suite as a non-root user (CI already
runs non-root, so CI needs no special handling):

    sudo -u ubuntu env HOME=/home/ubuntu PATH="/usr/local/go/bin:/usr/bin:/bin" \
        go test ./test/e2e/ -count=1 -v

The harness (`backend/test/support`) boots one embedded Postgres per
`go test` binary, gives each test an isolated migrated database, runs the audit
logger with test-fast flush intervals, and installs a dev RSA signing key + KEK.
Tests must not call `t.Parallel()`: the audit logger and the `db` package are
process-global singletons rebound per test.

## Not covered here

`auth.login` / `auth.login_failed` fire inside the GitHub device-code exchange,
which the harness deliberately does not drive (tokens are crafted directly via
`testsupport.NewAccount`). They are left as explicit red TODOs until a login seam
exists.
