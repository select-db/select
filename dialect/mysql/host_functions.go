package mysql

import (
	"strings"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/selectDb/dialect/mysql/parser"
)

// hostFunctions reach past the rows into the server's filesystem. A statement
// calling one is not the plain read its shape suggests, so it takes manage as
// well as whatever it reads.
//
// MySQL gates these on the FILE privilege, which is a second gate, not this
// one: a connection whose credentials we hold is often privileged enough.
var hostFunctions = map[string]bool{
	"load_file": true,
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
		if !hostFunctions[strings.ToLower(strings.Trim(all[ti].GetText(), "`"))] {
			continue
		}
		for next := ti + 1; next < to; next++ {
			if all[next].GetChannel() != antlr.TokenDefaultChannel {
				continue
			}
			if all[next].GetTokenType() == mysql.MySQLLexerOPEN_PAR_SYMBOL {
				return true
			}
			break
		}
	}
	return false
}
