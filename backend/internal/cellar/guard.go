package cellar

import (
	"errors"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	"github.com/selectDb/dialect/sqlite"
	parser "github.com/selectDb/dialect/sqlite/parser"
)

// ErrForbiddenStatement is a statement managed databases never run.
var ErrForbiddenStatement = errors.New("statement not allowed on managed databases")

// readOnlyPragmas are the PRAGMAs a user may run, as statements or as pragma_*
// table functions: they read the caller's own schema. The rest change
// connection or process state; temp_store_directory is process-wide.
var readOnlyPragmas = map[string]bool{
	"table_info":       true,
	"table_xinfo":      true,
	"table_list":       true,
	"index_list":       true,
	"index_info":       true,
	"index_xinfo":      true,
	"foreign_key_list": true,
}

// CheckStatement refuses sql that names a PRAGMA outside readOnlyPragmas.
func CheckStatement(sql string) error {
	lexer := parser.NewSQLiteLexer(antlr.NewInputStream(sql))
	lexer.RemoveErrorListeners()
	var toks []antlr.Token
	for _, t := range lexer.GetAllTokens() {
		if t.GetChannel() == antlr.TokenDefaultChannel {
			toks = append(toks, t)
		}
	}
	for i, t := range toks {
		var name string
		switch t.GetTokenType() {
		case parser.SQLiteLexerPRAGMA_:
			name = pragmaName(toks[i+1:])
		case parser.SQLiteLexerIDENTIFIER:
			var ok bool
			if name, ok = strings.CutPrefix(normalize(t.GetText()), "pragma_"); !ok {
				continue
			}
		default:
			continue
		}
		if !readOnlyPragmas[name] {
			return ErrForbiddenStatement
		}
	}
	return nil
}

// pragmaName reads `[schema.]name` after PRAGMA; "" when absent.
func pragmaName(rest []antlr.Token) string {
	if len(rest) >= 3 && rest[1].GetTokenType() == parser.SQLiteLexerDOT {
		rest = rest[2:]
	}
	if len(rest) == 0 {
		return ""
	}
	return normalize(rest[0].GetText())
}

var dialect = sqlite.NewDialect()

// normalize reads a name as SQLite does; PRAGMA also takes one as a string.
func normalize(s string) string {
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return strings.ToLower(strings.ReplaceAll(s[1:len(s)-1], "''", "'"))
	}
	return dialect.NormalizeIdentifier(s)
}
