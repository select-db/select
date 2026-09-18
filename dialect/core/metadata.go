package core

// Metadata is an in-memory catalog provided by the caller
type Metadata struct {
	DefaultDB     string
	DefaultSchema string
	CurrentSchema string // Current active schema for the user/session (empty means use DefaultSchema)
	Schemas       []Schema
}

// Type is a database type (from introspection) for casts, DDL, and hover.
type Type struct {
	Schema      string   `json:"schema"`
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	Display     string   `json:"display"`
	Description string   `json:"description,omitempty"`
	EnumLabels  []string `json:"enumLabels,omitempty"`
}

// Function is a callable routine (from introspection).
type Function struct {
	Schema      string `json:"schema"`
	Name        string `json:"name"`
	Args        string `json:"args"`
	Result      string `json:"result"`
	Kind        string `json:"kind"`
	Description string `json:"description,omitempty"`
	OID         int64  `json:"oid"`
}

// Schema holds schema-level objects
type Schema struct {
	Name              string
	Tables            []Table
	ForeignTables     []Table
	Views             []Table
	MaterializedViews []Table
	Indexes           []IndexInfo
	Triggers          []TriggerInfo
	Stats             TableStats
	Types             []Type
	Functions         []Function
	Settings          []Setting
}

type Setting struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Value       string `json:"value,omitempty"`
}

// AllTypes returns types across all schemas (catalog + user).
func (m *Metadata) AllTypes() []Type {
	var all []Type
	for _, s := range m.Schemas {
		all = append(all, s.Types...)
	}
	return all
}

// AllFunctions returns functions across all schemas (catalog + user).
func (m *Metadata) AllFunctions() []Function {
	var all []Function
	for _, s := range m.Schemas {
		all = append(all, s.Functions...)
	}
	return all
}

// Table represents a relation with columns
type Table struct {
	Name        string
	Columns     []Column
	PrimaryKey  []string // Column names that make up the primary key (empty for views or tables without PK)
	DDL         string   // DDL statement for the table/view (empty if not available)
	Description string   `json:",omitempty"` // Table COMMENT, if any (used for hover docs and schema-driven codegen)
}

// ForeignKeyRef describes the referenced table and column for a foreign key.
// Present only when the column is part of a foreign key constraint.
type ForeignKeyRef struct {
	SchemaName string
	TableName  string
	ColumnName string
}

// Column represents a column in a table or CTE
type Column struct {
	Name         string
	Type         string // Native column type e.g. varchar(255), int4, timestamptz
	Nullable     bool
	Default      *string // Default value expression, nil if none
	IsPrimaryKey bool
	IsForeignKey bool
	ForeignKey   *ForeignKeyRef // Set only when IsForeignKey is true
	// EnumValues holds the allowed values when the column is an enumerated
	// type (PG native enum, MySQL enum/set). Resolved once by
	// EnrichEnumValues; nil for every other type. Single source of truth
	// for the cell editor and the enum lint rule.
	EnumValues []string
	Extra      map[string]any // Dialect-specific metadata not covered above (optional)
	// Description is the column COMMENT, if any. Used for hover docs and as the
	// carrier for schema-driven codegen annotations.
	Description string `json:",omitempty"`
}

// IndexInfo represents an index in the database (dialect-agnostic)
type IndexInfo struct {
	Name      string
	TableName string
	DDL       string
	Columns   []IndexColumnInfo
}

// IndexColumnInfo represents a column in an index (dialect-agnostic)
// Fields are optional where not all databases support them
type IndexColumnInfo struct {
	Name       string
	Position   int    // Position/order of column in the index (1-based, similar to SeqNo)
	Collation  string // Collation name if specified (empty if not applicable)
	Descending bool   // Whether column is ordered descending (false = ascending/default)
}

// TriggerInfo represents a trigger in the database
type TriggerInfo struct {
	Name      string
	TableName string
	DDL       string
}

// TableStats represents statistics for a table or index
type TableStats map[string]string // table/index name -> stat value

// EnrichColumnsWithConstraints sets IsPrimaryKey, IsForeignKey, and ForeignKey on each column in place.
// pk is the list of primary key column names; fk maps local column name to the referenced (schema, table, column).
func EnrichColumnsWithConstraints(columns *[]Column, pk []string, fk map[string]ForeignKeyRef) {
	pkSet := make(map[string]bool, len(pk))
	for _, name := range pk {
		pkSet[name] = true
	}
	for i := range *columns {
		col := &(*columns)[i]
		col.IsPrimaryKey = pkSet[col.Name]
		if ref, ok := fk[col.Name]; ok {
			col.IsForeignKey = true
			col.ForeignKey = &ref
		}
	}
}
