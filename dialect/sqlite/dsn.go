package sqlite

import (
	"net/url"
	"strings"
)

// withDefensiveMode turns on SQLite's defensive connection mode for DSNs that
// have not already made a choice about it.
//
// Select opens database files it did not create. Defensive mode disables the
// SQL-level features that let an ordinary statement corrupt such a file:
// PRAGMA writable_schema=ON, PRAGMA schema_version=N and PRAGMA journal_mode=OFF
// become silent no-ops, and writes to a virtual table's shadow tables (fts5's
// _data, _idx and so on) and to sqlite_dbpage fail. Reading those tables,
// ordinary use of the virtual tables that own them, and VACUUM are unaffected,
// so this is invisible to everything a user actually does with a database.
//
// Two limits are worth stating, since the name invites more confidence than the
// flag earns. It is hardening, not a sandbox for hostile database files: this
// build compiles with neither SQLITE_TRUSTED_SCHEMA=0 nor SQLITE_DQS=0 and
// installs no authorizer. And it is a property of the connection, not of the
// file, so a handle opened elsewhere without it is unrestricted.
//
// The DSN is returned unchanged, leaving the driver's default behaviour in
// place, when:
//   - it already carries _defensive, so an explicit _defensive=0 opts out;
//   - it selects journal_mode=OFF, which the driver rejects outright in
//     combination with _defensive rather than silently honouring neither;
//   - its query string does not parse, leaving the driver to report why.
//
// The test for "already has a query string" is the driver's own (a '?' at index
// 1 or later), so this never produces a DSN that the driver reads differently
// than it does here.
func withDefensiveMode(dsn string) string {
	sep := "?"
	query := ""
	if pos := strings.IndexRune(dsn, '?'); pos >= 1 {
		sep = "&"
		query = dsn[pos+1:]
	}

	q, err := url.ParseQuery(query)
	if err != nil {
		return dsn
	}
	if _, ok := q["_defensive"]; ok {
		return dsn
	}
	if journalModeOff(q) {
		return dsn
	}

	return dsn + sep + "_defensive=1"
}

// journalModeOff reports whether the DSN selects journal_mode=OFF, resolving
// the _journal_mode/_journal pair the way the driver does: selection is by
// presence and the alias wins when both are given, so "_journal_mode=WAL&_journal="
// selects the alias and yields an empty value rather than falling back to WAL.
func journalModeOff(q url.Values) bool {
	mode := ""
	if _, ok := q["_journal_mode"]; ok {
		mode = q.Get("_journal_mode")
	}
	if _, ok := q["_journal"]; ok {
		mode = q.Get("_journal")
	}
	return strings.EqualFold(mode, "OFF")
}
