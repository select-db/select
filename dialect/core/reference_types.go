package core

// RelationRef is the unified scoped reference model used across the dialect layer.
// IsVirtual distinguishes derived relations (CTE/subquery aliases) from physical refs.
type RelationRef struct {
	Database      string
	Schema        string
	Table         string
	Alias         string
	Columns       []Column
	IsVirtual     bool
	ScopeStartPos int // Token position where this ref becomes available (-1 if not tracked)
	ScopeEndPos   int // Token position where this ref goes out of scope (-1 for end of query)
	NestingLevel  int // Nesting level where this ref was defined (0 = main query, 1 = first subquery, etc.)
	// Source position of the table name token (1-based line, 0-based col, exclusive endCol).
	Line   int
	Col    int
	EndCol int
}

// ColumnAlias represents a column alias defined in the SELECT list.
type ColumnAlias struct {
	Alias    string
	Column   string
	Type     string
	Schema   string
	Table    string
	Nullable bool
}
