package sqlite

import (
	"strings"

	"github.com/antlr4-go/antlr/v4"
	sqlite "github.com/selectDb/dialect/sqlite/parser"
)

// hostFunctions reach past the rows into the filesystem. They come from the
// fileio extension rather than the core build, so a connection only has them
// where someone loaded it: that is a second gate, not this one. A statement
// calling one is not the plain read its shape suggests.
var hostFunctions = map[string]bool{
	"readfile":  true,
	"writefile": true,
	"fsdir":     true,
	"edit":      true,
	"lsmode":    true,
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
		if all[ti].GetChannel() != antlr.TokenDefaultChannel {
			continue
		}
		if !hostFunctions[strings.ToLower(strings.Trim(all[ti].GetText(), "\"`[]"))] {
			continue
		}
		for next := ti + 1; next < to; next++ {
			if all[next].GetChannel() != antlr.TokenDefaultChannel {
				continue
			}
			if all[next].GetTokenType() == sqlite.SQLiteParserOPEN_PAR {
				return true
			}
			break
		}
	}
	return false
}
