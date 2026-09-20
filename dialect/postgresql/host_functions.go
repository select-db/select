package postgresql

import (
	"strings"

	"github.com/antlr4-go/antlr/v4"
	pg "github.com/selectDb/dialect/postgresql/parser"
)

// hostFunctions reach past the rows into the server itself: its filesystem, its
// large objects, another server. A statement calling one is not the plain read
// its shape suggests, so it takes manage as well as whatever it reads.
//
// The database grants these separately, to a superuser or a role in
// pg_read_server_files and friends. That is a second gate, not this one: a
// connection whose credentials we hold is often privileged enough.
var hostFunctions = map[string]bool{
	"pg_read_file":         true,
	"pg_read_binary_file":  true,
	"pg_write_file":        true,
	"pg_file_write":        true,
	"pg_file_rename":       true,
	"pg_file_unlink":       true,
	"pg_ls_dir":            true,
	"pg_logdir_ls":         true,
	"pg_stat_file":         true,
	"lo_import":            true,
	"lo_export":            true,
	"dblink":               true,
	"dblink_connect":       true,
	"dblink_exec":          true,
	"dblink_connect_u":     true,
	"pg_reload_conf":       true,
	"pg_rotate_logfile":    true,
	"pg_terminate_backend": true,
}

// callsHostFunction reports whether the tokens between from and to name one of
// those in a call position. Reading the tokens rather than the tree keeps this
// working on a statement error recovery left incomplete.
func callsHostFunction(tokens *antlr.CommonTokenStream, from, to int) bool {
	all := tokens.GetAllTokens()
	if to > len(all) {
		to = len(all)
	}
	for ti := from; ti < to; ti++ {
		if !isCallTo(all, ti, to) {
			continue
		}
		if hostFunctions[normalizeCallName(all[ti].GetText())] {
			return true
		}
	}
	return false
}

// isCallTo reports whether the token at ti is an identifier the next default
// token opens a call on.
func isCallTo(all []antlr.Token, ti, to int) bool {
	if all[ti].GetChannel() != antlr.TokenDefaultChannel {
		return false
	}
	for next := ti + 1; next < to; next++ {
		if all[next].GetChannel() != antlr.TokenDefaultChannel {
			continue
		}
		return all[next].GetTokenType() == pg.PostgreSQLParserOPEN_PAREN
	}
	return false
}

// normalizeCallName lowercases and strips the quoting a call name may carry.
func normalizeCallName(text string) string {
	if len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' {
		text = text[1 : len(text)-1]
	}
	return strings.ToLower(text)
}
