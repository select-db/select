package mysql

import (
	"github.com/antlr4-go/antlr/v4"
	"github.com/selectDb/dialect/core"
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

// callsHostFunction reports whether the tokens between from and to call one of
// them.
func callsHostFunction(tokens *antlr.CommonTokenStream, from, to int) bool {
	return core.CallsAnyOf(tokens, from, to, hostFunctions, "`")
}
