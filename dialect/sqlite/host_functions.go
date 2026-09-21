package sqlite

import (
	"github.com/antlr4-go/antlr/v4"
	"github.com/selectDb/dialect/core"
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

// callsHostFunction reports whether the tokens between from and to call one of
// them.
func callsHostFunction(tokens *antlr.CommonTokenStream, from, to int) bool {
	return core.CallsAnyOf(tokens, from, to, hostFunctions, "\"`[]")
}
