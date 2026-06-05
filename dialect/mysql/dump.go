package mysql

import (
	"os"
	"os/exec"
	"strings"

	"github.com/selectDb/dialect/core"
)

// DumpSchema runs mysqldump --no-data against the given MySQL DSN.
// Returns (sql, true) on success, ("", false) when mysqldump is not installed or fails.
func (d *Dialect) DumpSchema(dsn string) (string, bool) {
	toolPath, err := exec.LookPath("mysqldump")
	if err != nil {
		return "", false
	}
	user, pass, host, port, dbname, ok := core.MySQLConnParams(dsn)
	if !ok {
		return "", false
	}
	// A field beginning with '-' would be parsed by mysqldump as a flag
	// (argument injection); '--' also ends option parsing before dbname.
	for _, v := range []string{host, port, user, dbname} {
		if strings.HasPrefix(v, "-") {
			return "", false
		}
	}
	args := []string{
		"--no-data", "--routines", "--triggers", "--no-tablespaces",
		"-h", host, "-P", port, "-u", user, "--", dbname,
	}
	cmd := exec.Command(toolPath, args...)
	if pass != "" {
		cmd.Env = append(os.Environ(), "MYSQL_PWD="+pass)
	}
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

// DumpSchemaWarning returns a SQL comment block warning when mysqldump is unavailable.
func (d *Dialect) DumpSchemaWarning() string {
	return "-- WARNING: mysqldump was not found on this machine.\n" +
		"-- This schema was reconstructed from MySQL information_schema queries.\n" +
		"-- It may be missing stored procedures, functions, triggers, and events.\n" +
		"-- Install mysqldump (mysql-client package) for an authoritative dump.\n\n"
}
