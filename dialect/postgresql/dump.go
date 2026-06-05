package postgresql

import (
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/selectDb/dialect/core"
)

// resolveToolPath finds a CLI tool by first trying exec.LookPath (uses the process
// PATH) and, if that fails, asking the OS shell to resolve it. GUI apps on all
// platforms may not inherit the full login-shell PATH, so the shell fallback ensures
// tools installed via package managers (Homebrew, apt, winget…) are still found.
var commonToolDirs = []string{
	"/opt/homebrew/bin",
	"/usr/local/bin",
	"/usr/bin",
}

func resolveToolPath(name string) (string, bool) {
	if p, err := exec.LookPath(name); err == nil {
		return p, true
	}

	// GUI apps on macOS don't inherit the shell PATH. Try the login shell.
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "where", name)
	} else {
		cmd = exec.Command("/bin/sh", "-lc", "which "+name)
	}
	if out, err := cmd.Output(); err == nil {
		if p := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0]); p != "" {
			return p, true
		}
	}

	// Last resort: check common install directories directly.
	for _, dir := range commonToolDirs {
		p := dir + "/" + name
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}

	return "", false
}

// DumpSchema runs pg_dump --schema-only against the given DSN.
// Returns (sql, true) on success, ("", false) when pg_dump is not installed or fails.
func (d *Dialect) DumpSchema(dsn string) (string, bool) {
	toolPath, ok := resolveToolPath("pg_dump")
	if !ok {
		return "", false
	}
	// Discrete args, not -d <dsn>: a libpq connstring honours sslkey=/sslcert=/
	// passfile=/service=/options= (arbitrary file read). Host is already pinned
	// by engine.ResolveDumpDSN so pg_dump can't re-resolve.
	user, pass, host, port, dbname, ok := core.PostgresConnParams(dsn)
	if !ok {
		return "", false
	}
	// A leading '-' would be read as a flag
	for _, v := range []string{host, port, user, dbname} {
		if strings.HasPrefix(v, "-") {
			return "", false
		}
	}
	args := []string{
		"--schema-only", "--no-owner", "--no-privileges",
		"-h", host, "-p", port, "-U", user, "-d", dbname,
	}
	cmd := exec.Command(toolPath, args...)
	cmd.Env = append(os.Environ(), "PGCONNECT_TIMEOUT=10")
	if pass != "" {
		cmd.Env = append(cmd.Env, "PGPASSWORD="+pass)
	}
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

// DumpSchemaWarning returns a SQL comment block warning when DumpSchema is unavailable
// or fails (e.g. pg_dump not installed, or client/server version mismatch).
func (d *Dialect) DumpSchemaWarning() string {
	return "-- WARNING: pg_dump could not produce an authoritative dump for this database.\n" +
		"-- This schema was reconstructed from PostgreSQL catalog queries.\n" +
		"-- It may be missing sequences, extensions, grants, stored functions,\n" +
		"-- custom types, and other database objects.\n" +
		"-- Ensure pg_dump is installed and matches your server version for a full dump.\n\n"
}
