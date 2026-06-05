package sqllang

import (
	"regexp"
	"strings"

	"selectDb/backend/utils"
)

// VarReplacer resolves a single SQL variable reference.
type VarReplacer interface {
	ResolveVariable(varName, folderID string) (value string, isSqlFile bool, err error)
}

var varPattern = regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)`)

type overrideReplacer struct {
	overrides map[string]string
	fallback  VarReplacer
}

func (r *overrideReplacer) ResolveVariable(varName, folderID string) (string, bool, error) {
	if v, ok := r.overrides[varName]; ok {
		return v, false, nil
	}
	return r.fallback.ResolveVariable(varName, folderID)
}

// WithOverrides returns a VarReplacer that checks overrides first, then falls through to fallback.
func WithOverrides(overrides map[string]string, fallback VarReplacer) VarReplacer {
	return &overrideReplacer{overrides: overrides, fallback: fallback}
}

type subRepl struct {
	Start, End int
	New        string
}

// SubstituteVariables replaces $VARIABLE patterns with values from the resolver (no SQL quoting).
func SubstituteVariables(vr VarReplacer, input, folderID string) (string, error) {
	res, _, err := substituteVariablesInternalWithRepl(vr, input, folderID, false, nil)
	return res, err
}

// SubstituteVariablesSQL replaces $VARIABLE patterns with properly escaped SQL values.
func SubstituteVariablesSQL(vr VarReplacer, input, folderID string) (string, error) {
	res, _, err := substituteVariablesInternalWithRepl(vr, input, folderID, true, nil)
	return res, err
}

// SubstituteVariablesSQLWithCaret resolves variables and returns the resolved SQL plus the
// updated caret position (1-based line, 0-based offset).
func SubstituteVariablesSQLWithCaret(vr VarReplacer, input, folderID string, caretLine, caretOffset int) (resolved string, newCaretLine, newCaretOffset int, err error) {
	visiting := make(map[string]bool)
	resolved, repls, err := substituteVariablesInternalWithRepl(vr, input, folderID, true, visiting)
	if err != nil {
		return "", caretLine, caretOffset, err
	}
	newCaretLine, newCaretOffset = trackCaretThroughReplacements(input, resolved, repls, caretLine, caretOffset)
	return resolved, newCaretLine, newCaretOffset, nil
}

func substituteVariablesInternalWithRepl(vr VarReplacer, input, folderID string, sqlContext bool, visiting map[string]bool) (string, []subRepl, error) {
	if visiting == nil {
		visiting = make(map[string]bool)
	}
	var repls []subRepl
	matches := varPattern.FindAllStringSubmatchIndex(input, -1)
	if len(matches) == 0 {
		return input, nil, nil
	}
	var result strings.Builder
	var lastEnd int
	for _, m := range matches {
		start, end := m[0], m[1]
		varName := input[m[2]:m[3]]
		value, isSqlFile, err := vr.ResolveVariable(varName, folderID)
		if err != nil {
			return "", nil, err
		}
		var newText string
		if isSqlFile {
			if visiting[varName] {
				newText = input[start:end]
			} else {
				visiting[varName] = true
				expanded, _, err := substituteVariablesInternalWithRepl(vr, value, folderID, true, visiting)
				delete(visiting, varName)
				if err != nil {
					return "", nil, err
				}
				newText = expanded
			}
		} else if sqlContext {
			newText = escapeSQLValue(value)
		} else {
			newText = value
		}
		result.WriteString(input[lastEnd:start])
		result.WriteString(newText)
		repls = append(repls, subRepl{Start: start, End: end, New: newText})
		lastEnd = end
	}
	result.WriteString(input[lastEnd:])
	return result.String(), repls, nil
}

func escapeSQLValue(value string) string {
	if utils.IsNumeric(value) {
		return value
	}
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "true" || lower == "false" {
		return strings.ToUpper(lower)
	}
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func trackCaretThroughReplacements(original, resolved string, repls []subRepl, caretLine, caretOffset int) (newLine, newCol int) {
	idx := lineColToIndex(original, caretLine, caretOffset)
	for _, r := range repls {
		if r.End <= idx {
			idx += len(r.New) - (r.End - r.Start)
		} else if r.Start < idx {
			idx = r.Start
		}
	}
	return indexToLineCol(resolved, idx)
}

func lineColToIndex(s string, line, col int) int {
	lines := strings.Split(s, "\n")
	if line < 1 || line > len(lines) {
		return 0
	}
	idx := 0
	for i := 0; i < line-1; i++ {
		idx += len(lines[i]) + 1
	}
	if col > len(lines[line-1]) {
		col = len(lines[line-1])
	}
	return idx + col
}

func indexToLineCol(s string, index int) (line, col int) {
	if index <= 0 {
		return 1, 0
	}
	if index >= len(s) {
		index = len(s)
	}
	lines := strings.Split(s, "\n")
	line = 1
	for _, l := range lines {
		lineLen := len(l) + 1
		if index < lineLen {
			if index > len(l) {
				col = len(l)
			} else {
				col = index
			}
			return line, col
		}
		index -= lineLen
		line++
	}
	return line, col
}
