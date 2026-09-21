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

// namesARelation are the keywords a table name follows. A parenthesis after
// the name is then the column list or the alias list, not a call, so "INSERT
// INTO readfile (c1) VALUES (1)" is the ordinary insert it looks like. FROM and
// JOIN are absent on purpose: a table-valued function is called there.
var namesARelation = map[string]bool{
	"into":   true,
	"update": true,
	"table":  true,
	"as":     true,
}

// CallsAnyOf reports whether the tokens in [from, to) name one of names in a
// call position: a default-channel token the next default-channel token opens
// a parenthesis on, where the one before it did not introduce a relation.
// Reading the tokens rather than the tree keeps this working on a statement
// error recovery left incomplete.
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
		if nextDefaultToken(all, ti+1, to) != "(" {
			continue
		}
		if namesARelation[strings.ToLower(previousDefaultToken(all, from, ti))] {
			continue
		}
		return true
	}
	return false
}

// PrecededBy reports whether the default-channel token before the one at idx
// has the given text, compared case-insensitively.
func PrecededBy(all []antlr.Token, from, idx int, text string) bool {
	return strings.EqualFold(previousDefaultToken(all, from, idx), text)
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

// previousDefaultToken returns the text of the last default-channel token in
// [from, idx), or the empty string when there is none.
func previousDefaultToken(all []antlr.Token, from, idx int) string {
	for ti := idx - 1; ti >= from; ti-- {
		if all[ti].GetChannel() == antlr.TokenDefaultChannel {
			return all[ti].GetText()
		}
	}
	return ""
}

// NodeSpan is the half-open token span a node covers by itself, as opposed to
// TokenSpan, which runs to the next node so a scan reaches a clause the node
// does not own. ok is false for a node error recovery left without bounds.
func NodeSpan(node interface {
	GetStart() antlr.Token
	GetStop() antlr.Token
}) (from, to int, ok bool) {
	if node == nil {
		return 0, 0, false
	}
	start, stop := node.GetStart(), node.GetStop()
	if start == nil || stop == nil {
		return 0, 0, false
	}
	return start.GetTokenIndex(), stop.GetTokenIndex() + 1, true
}
