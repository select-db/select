package core

// RelationRef is the unified scoped reference model used across the dialect layer.
// IsVirtual distinguishes derived relations (CTE/subquery aliases) from physical
// refs. Where a name is matched against the declared ones instead, Qualified
// decides first: only an unqualified reference can be a CTE or an alias.
type RelationRef struct {
	Database  string
	Schema    string
	Table     string
	Alias     string
	Columns   []Column
	IsVirtual bool
	// Qualified means the SQL carried a schema. Schema cannot say so on its own:
	// an unqualified name is given the default one, so t3 and main.t3 both
	// arrive as main.
	Qualified     bool
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
