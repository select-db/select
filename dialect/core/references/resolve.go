package core_references

import core "github.com/selectDb/dialect/core"

// CaretCharOffset converts a 1-based line and 0-based column to a character offset.
func CaretCharOffset(sql string, line, col int) int {
	currentLine := 1
	for i, ch := range sql {
		if currentLine == line {
			return i + col
		}
		if ch == '\n' {
			currentLine++
		}
	}
	return len(sql)
}

func pickInnermostRelationRef(refs []*RelationRef) *RelationRef {
	if len(refs) == 0 {
		return nil
	}
	best := refs[0]
	for _, r := range refs[1:] {
		if r.NestingLevel > best.NestingLevel {
			best = r
		}
	}
	return best
}

func pickInnermostVirtualTable(vtabs []*RelationRef) *RelationRef {
	if len(vtabs) == 0 {
		return nil
	}
	best := vtabs[0]
	for _, vt := range vtabs[1:] {
		if vt.NestingLevel > best.NestingLevel {
			best = vt
		}
	}
	return best
}

func visibleRelationRefMatches(refs []RelationRef, name string, caretIdx int, norm func(string) string, aliasOnly bool) []*RelationRef {
	nn := norm(name)
	var matches []*RelationRef
	for i := range refs {
		r := &refs[i]
		if !core.ScopeContainsCaret(r.ScopeStartPos, r.ScopeEndPos, caretIdx) {
			continue
		}
		if r.Alias != "" && norm(r.Alias) == nn {
			matches = append(matches, r)
			continue
		}
		if aliasOnly {
			continue
		}
		if norm(r.Table) == nn {
			matches = append(matches, r)
		}
	}
	return matches
}

// RelationRefForQualifier resolves the left side of qual.column (alias or table name), same preference as completion.
func RelationRefForQualifier(refs []RelationRef, qualifier string, caretIdx int, norm func(string) string) *RelationRef {
	if norm == nil {
		norm = func(s string) string { return s }
	}
	aliasMatches := visibleRelationRefMatches(refs, qualifier, caretIdx, norm, true)
	if len(aliasMatches) > 0 {
		return pickInnermostRelationRef(aliasMatches)
	}
	tableMatches := visibleRelationRefMatches(refs, qualifier, caretIdx, norm, false)
	// Remove alias matches from the second pass to preserve alias-first behavior.
	if len(aliasMatches) > 0 && len(tableMatches) > 0 {
		filtered := make([]*RelationRef, 0, len(tableMatches))
		for _, t := range tableMatches {
			if t.Alias != "" && norm(t.Alias) == norm(qualifier) {
				continue
			}
			filtered = append(filtered, t)
		}
		tableMatches = filtered
	}
	if len(tableMatches) > 0 {
		return pickInnermostRelationRef(tableMatches)
	}
	return nil
}

// RelationRefForSingleIdent resolves a bare identifier: alias first, then table name in FROM scope.
func RelationRefForSingleIdent(refs []RelationRef, ident string, caretIdx int, norm func(string) string) *RelationRef {
	if norm == nil {
		norm = func(s string) string { return s }
	}
	aliasMatches := visibleRelationRefMatches(refs, ident, caretIdx, norm, true)
	if len(aliasMatches) > 0 {
		return pickInnermostRelationRef(aliasMatches)
	}
	tableMatches := visibleRelationRefMatches(refs, ident, caretIdx, norm, false)
	if len(tableMatches) > 0 {
		filtered := make([]*RelationRef, 0, len(tableMatches))
		for _, t := range tableMatches {
			if t.Alias != "" && norm(t.Alias) == norm(ident) {
				continue
			}
			filtered = append(filtered, t)
		}
		tableMatches = filtered
	}
	return pickInnermostRelationRef(tableMatches)
}

// VirtualTableForQualifier returns the innermost virtual table (CTE / derived) whose name matches
// qualifier and whose outer scope contains the caret.
func VirtualTableForQualifier(vtabs []RelationRef, qualifier string, caretIdx int, norm func(string) string) *RelationRef {
	if norm == nil {
		norm = func(s string) string { return s }
	}
	nq := norm(qualifier)
	var matches []*RelationRef
	for i := range vtabs {
		vt := &vtabs[i]
		if norm(vt.Table) != nq {
			continue
		}
		if !core.ScopeContainsCaret(vt.ScopeStartPos, vt.ScopeEndPos, caretIdx) {
			continue
		}
		matches = append(matches, vt)
	}
	return pickInnermostVirtualTable(matches)
}
