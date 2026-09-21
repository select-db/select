package core

import (
	"strings"

	"github.com/antlr4-go/antlr/v4"
)

// TokenSpan is the half-open token range of nodes[idx], bounded by the next
// node so a script does not leak one statement's tokens into another. A node
// error recovery left without a start token falls back to the script's own
// bounds.
func TokenSpan[T interface{ GetStart() antlr.Token }](
	tokens *antlr.CommonTokenStream,
	nodes []T,
	idx int,
) (from, to int) {
	to = len(tokens.GetAllTokens())
	if idx < 0 || idx >= len(nodes) {
		return 0, to
	}
	if node := nodes[idx]; any(node) != nil {
		if start := node.GetStart(); start != nil {
			from = start.GetTokenIndex()
		}
	}
	for _, next := range nodes[idx+1:] {
		if any(next) == nil {
			continue
		}
		if start := next.GetStart(); start != nil {
			to = start.GetTokenIndex()
		}
		break
	}
	return from, to
}

// CallsAnyOf reports whether the tokens in [from, to) name one of names in a
// call position: a default-channel token the next default-channel token opens
// a parenthesis on. Reading the tokens rather than the tree keeps this working
// on a statement error recovery left incomplete.
//
// quoting is the set of characters the dialect quotes an identifier with. A
// name is matched unquoted and lowercased, so names holds bare lowercase
// entries.
func CallsAnyOf(
	tokens *antlr.CommonTokenStream,
	from, to int,
	names map[string]bool,
	quoting string,
) bool {
	all := tokens.GetAllTokens()
	to = Clamp(to, 0, len(all))
	from = Clamp(from, 0, to)
	for ti := from; ti < to; ti++ {
		if all[ti].GetChannel() != antlr.TokenDefaultChannel {
			continue
		}
		if !names[strings.ToLower(strings.Trim(all[ti].GetText(), quoting))] {
			continue
		}
		if nextDefaultToken(all, ti+1, to) == "(" {
			return true
		}
	}
	return false
}

// nextDefaultToken returns the text of the first default-channel token in
// [from, to), or the empty string when there is none.
func nextDefaultToken(all []antlr.Token, from, to int) string {
	for ti := from; ti < to; ti++ {
		if all[ti].GetChannel() == antlr.TokenDefaultChannel {
			return all[ti].GetText()
		}
	}
	return ""
}
