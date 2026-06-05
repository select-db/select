package sqllang

// PositionParams identifies a caret position inside a SQL file/buffer.
type PositionParams struct {
	DbInstanceID string
	FileID       string // optional; enables variable substitution
	SQL          string
	Line         int
	Column       int
}

// resolveEditorPosition substitutes variables and returns the resolved SQL + caret.
func (s *SqlLang) resolveEditorPosition(p PositionParams) (sql string, line, col int) {
	sql, line, col = p.SQL, p.Line, p.Column
	if p.FileID == "" {
		return
	}
	file := s.graph.GetFileNodeByID(p.FileID)
	if file == nil || file.FolderID == "" {
		return
	}
	resolved, nl, nc, err := SubstituteVariablesSQLWithCaret(s.graph, p.SQL, file.FolderID, p.Line, p.Column)
	if err != nil {
		return
	}
	return resolved, nl, nc
}
