package core

import (
	"fmt"
	"sort"
	"strings"
)

// ============================================
// UTILITIES - Dialect-agnostic helpers
// ============================================

// SplitQualified splits a qualified name respecting quoted identifiers.
// This is a dialect-agnostic utility that can be used by any SQL dialect.
func SplitQualified(s string, quoteChar rune) []string {
	var parts []string
	var buf []rune
	inQuote := false

	for i := 0; i < len(s); {
		r := rune(s[i])
		if r == quoteChar {
			buf = append(buf, r)
			i++
			inQuote = !inQuote
			for inQuote && i < len(s) {
				rr := rune(s[i])
				buf = append(buf, rr)
				i++
				if rr == quoteChar {
					if i < len(s) && rune(s[i]) == quoteChar {
						buf = append(buf, quoteChar)
						i++
						continue
					}
					inQuote = false
					break
				}
			}
			continue
		}
		if r == '.' && !inQuote {
			if part := strings.TrimSpace(string(buf)); part != "" {
				parts = append(parts, part)
			}
			buf = buf[:0]
			i++
			continue
		}
		buf = append(buf, r)
		i++
	}

	if len(buf) > 0 {
		if part := strings.TrimSpace(string(buf)); part != "" {
			parts = append(parts, part)
		}
	}

	return parts
}

// Clamp constrains value between min and max
func Clamp(val, min, max int) int {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// TableInfo stores information about table references and aliases
type TableInfo struct {
	OriginalSchema string
	OriginalTable  string
	IsAlias        bool
}

// ============================================
// METADATA OPERATIONS
// ============================================

// GetDefaultSchema returns the default schema name or "public"
func GetDefaultSchema(meta Metadata) string {
	if meta.DefaultSchema != "" {
		return meta.DefaultSchema
	}
	return "public"
}

// MetaToSchemaDict converts Metadata into a nested dict for the Python analyzer:
// { schema: { table: { column: type } } }
func MetaToSchemaDict(meta Metadata) map[string]map[string]map[string]string {
	result := make(map[string]map[string]map[string]string, len(meta.Schemas))
	for _, s := range meta.Schemas {
		tables := make(map[string]map[string]string)
		for _, t := range GetAllTablesFromSchema(s) {
			if t.Name == "" {
				continue
			}
			cols := make(map[string]string, len(t.Columns))
			for _, c := range t.Columns {
				if c.Name != "" {
					cols[c.Name] = c.Type
				}
			}
			tables[t.Name] = cols
		}
		result[s.Name] = tables
	}
	return result
}

// MetaToEnumDict projects the already-resolved Column.EnumValues into the
// same schema->table->column shape as MetaToSchemaDict. It is a pure
// projection of canonical metadata (the join lives in EnrichEnumValues),
// so the lint analyzer and the cell editor never diverge. Columns without
// enum values are omitted to keep the payload small.
func MetaToEnumDict(meta Metadata) map[string]map[string]map[string][]string {
	result := make(map[string]map[string]map[string][]string)
	for _, s := range meta.Schemas {
		tables := make(map[string]map[string][]string)
		for _, t := range GetAllTablesFromSchema(s) {
			if t.Name == "" {
				continue
			}
			cols := make(map[string][]string)
			for _, c := range t.Columns {
				if c.Name != "" && len(c.EnumValues) > 0 {
					cols[c.Name] = c.EnumValues
				}
			}
			if len(cols) > 0 {
				tables[t.Name] = cols
			}
		}
		if len(tables) > 0 {
			result[s.Name] = tables
		}
	}
	return result
}

// MetaToFunctionNames returns deduplicated lowercase function names from all schemas.
func MetaToFunctionNames(meta Metadata) []string {
	fns := meta.AllFunctions()
	names := make([]string, 0, len(fns))
	seen := make(map[string]bool, len(fns))
	for _, f := range fns {
		key := strings.ToLower(f.Name)
		if f.Name != "" && !seen[key] {
			seen[key] = true
			names = append(names, f.Name)
		}
	}
	return names
}

// ListAllSchemas returns all schema names sorted
func ListAllSchemas(meta Metadata) []string {
	res := make([]string, 0, len(meta.Schemas))
	for _, s := range meta.Schemas {
		res = append(res, s.Name)
	}
	sort.Strings(res)
	return res
}

// GetAllTablesFromSchema returns all table types from a schema
func GetAllTablesFromSchema(schema Schema) []Table {
	result := make([]Table, 0,
		len(schema.Tables)+len(schema.Views)+len(schema.MaterializedViews)+len(schema.ForeignTables))
	result = append(result, schema.Tables...)
	result = append(result, schema.Views...)
	result = append(result, schema.MaterializedViews...)
	result = append(result, schema.ForeignTables...)
	return result
}

// SchemaExists checks if a schema name exists in the metadata
func SchemaExists(meta Metadata, schemaName string, normalizer func(string) string) bool {
	normalized := normalizer(schemaName)
	for _, schema := range meta.Schemas {
		if normalizer(schema.Name) == normalized {
			return true
		}
	}
	return false
}

// ============================================
// CANDIDATE OPERATIONS
// ============================================

// DeduplicateAndSort removes duplicate candidates and sorts them
func DeduplicateAndSort(candidates []Candidate) []Candidate {
	if len(candidates) == 0 {
		return candidates
	}

	seen := make(map[string]Candidate, len(candidates))
	for _, c := range candidates {
		// Include Definition in the key to avoid deduplicating candidates with different contexts
		key := fmt.Sprintf("%d\x00%s\x00%s", c.Type, c.Text, c.Definition)
		seen[key] = c
	}

	// Convert map to slice and sort to ensure deterministic order
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result := make([]Candidate, 0, len(seen))
	for _, k := range keys {
		result = append(result, seen[k])
	}

	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.Type == CandidateTypeColumn != (b.Type == CandidateTypeColumn) {
			return a.Type == CandidateTypeColumn
		}
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		return a.Text < b.Text
	})

	return result
}

// ============================================
// LIST OPERATIONS
// ============================================

// ListRelationsByType lists relations of a specific type in a schema
func ListRelationsByType(meta Metadata, schema string, getter func(Schema) []Table) []string {
	for _, s := range meta.Schemas {
		if s.Name == schema {
			tables := getter(s)
			out := make([]string, 0, len(tables))
			for _, t := range tables {
				out = append(out, t.Name)
			}
			sort.Strings(out)
			return out
		}
	}
	return nil
}

// ListTables lists all tables in a schema
func ListTables(meta Metadata, schema string) []string {
	return ListRelationsByType(meta, schema, func(s Schema) []Table { return s.Tables })
}

// ListForeignTables lists all foreign tables in a schema
func ListForeignTables(meta Metadata, schema string) []string {
	return ListRelationsByType(meta, schema, func(s Schema) []Table { return s.ForeignTables })
}

// ListViews lists all views in a schema
func ListViews(meta Metadata, schema string) []string {
	return ListRelationsByType(meta, schema, func(s Schema) []Table { return s.Views })
}

// ListMaterializedViews lists all materialized views in a schema
func ListMaterializedViews(meta Metadata, schema string) []string {
	return ListRelationsByType(meta, schema, func(s Schema) []Table { return s.MaterializedViews })
}

// ListColumns returns columns for a specific table or view
func ListColumns(meta Metadata, schema, table string) []Column {
	for _, s := range meta.Schemas {
		if s.Name == schema {
			// Search in all table types (tables, views, materialized views, foreign tables)
			for _, t := range GetAllTablesFromSchema(s) {
				if t.Name == table {
					return t.Columns
				}
			}
		}
	}
	return nil
}

// ListRelations lists all tables and views in specified schemas
func ListRelations(
	meta Metadata,
	schemaSet map[string]bool,
	quoter func(string, bool, map[string]bool) string,
	caretQuoted bool,
	reserved map[string]bool,
) (tables, views []Candidate) {
	for s := range schemaSet {
		for _, t := range ListTables(meta, s) {
			tables = append(tables, Candidate{
				Type: CandidateTypeTable,
				Text: quoter(t, caretQuoted, reserved),
			})
		}
		for _, t := range ListForeignTables(meta, s) {
			tables = append(tables, Candidate{
				Type: CandidateTypeForeignTable,
				Text: quoter(t, caretQuoted, reserved),
			})
		}
		for _, v := range ListViews(meta, s) {
			views = append(views, Candidate{
				Type: CandidateTypeView,
				Text: quoter(v, caretQuoted, reserved),
			})
		}
		for _, v := range ListMaterializedViews(meta, s) {
			views = append(views, Candidate{
				Type: CandidateTypeMaterializedView,
				Text: quoter(v, caretQuoted, reserved),
			})
		}
	}
	return
}

// ============================================
// IDENTIFIER HELPERS
// ============================================

// SplitQualifiedIdentifier splits a qualified name like "schema.table.column"
// respecting quoted identifiers. Works for any SQL
//
// Parameters:
//   - s: the qualified identifier string
//   - quoteChar: the quote character ('"' for PostgreSQL/SQLite, '`' for MySQL)
//
// Examples:
//
//	SplitQualifiedIdentifier("public.users", '"')       -> ["public", "users"]
//	SplitQualifiedIdentifier(`"My Schema".table`, '"')  -> ["My Schema", "table"]
//	SplitQualifiedIdentifier("db.`table`", '`')         -> ["db", "table"]
//	SplitQualifiedIdentifier(`"a""b".c`, '"')           -> ["a"b", "c"]  (doubled quotes)
func SplitQualifiedIdentifier(s string, quoteChar rune) []string {
	var parts []string
	var buf []rune
	inQuote := false

	for i := 0; i < len(s); {
		r := rune(s[i])
		if r == quoteChar {
			buf = append(buf, r)
			i++
			inQuote = !inQuote

			// Read until closing quote, handling doubled quotes
			for inQuote && i < len(s) {
				rr := rune(s[i])
				buf = append(buf, rr)
				i++

				if rr == quoteChar {
					// Check for doubled quote (escape sequence)
					if i < len(s) && s[i] == byte(quoteChar) {
						buf = append(buf, quoteChar)
						i++
						continue
					}
					// Single quote = end of quoted identifier
					inQuote = false
					break
				}
			}
			continue
		}

		// Dot is separator only outside quotes
		if r == '.' && !inQuote {
			if part := strings.TrimSpace(string(buf)); part != "" {
				parts = append(parts, part)
			}
			buf = buf[:0]
			i++
			continue
		}

		buf = append(buf, r)
		i++
	}

	// Don't forget the last part
	if len(buf) > 0 {
		if part := strings.TrimSpace(string(buf)); part != "" {
			parts = append(parts, part)
		}
	}

	return parts
}

// NormalizeQualifiedName splits and normalizes a qualified identifier
//
// Parameters:
//   - s: the qualified identifier string
//   - quoteChar: the quote character for the dialect
//   - normalizer: function to normalize each part (handles quoting rules)
//
// Example:
//
//	NormalizeQualifiedName("Public.Users", '"', pgNormalizer)
//	-> ["public", "users"]  (lowercased unquoted identifiers)
func NormalizeQualifiedName(s string, quoteChar rune, normalizer func(string) string) []string {
	parts := SplitQualifiedIdentifier(s, quoteChar)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, normalizer(p))
	}
	return out
}

// FindColumnInRefs looks up a column's metadata from table references
// Returns schema, table, type, and nullable status
//
// This is used during SELECT list parsing to find column types
// for building accurate column alias metadata.
//
// Example:
//
//	refs = [{Schema: "public", Table: "users", ...}]
//	FindColumnInRefs("email", refs, meta, normalizer)
//	-> ("public", "users", "varchar", false)
func FindColumnInRefs(
	columnName string,
	refs []RelationRef,
	meta Metadata,
	normalizer func(string) string,
) (schema, table, colType string, nullable bool) {
	normalizedName := normalizer(columnName)

	for _, ref := range refs {
		columns := GetColumnsForTable(meta, ref.Schema, ref.Table, normalizer)
		for _, col := range columns {
			if normalizer(col.Name) == normalizedName {
				return ref.Schema, ref.Table, col.Type, col.Nullable
			}
		}
	}

	return "unknown", "unknown", "unknown", true
}

// GetColumnsForTable retrieves columns for a table with normalization
// This replaces the simpler ListColumns() for cases where normalization is needed
//
// Parameters:
//   - meta: the metadata catalog
//   - schemaName: schema name (empty string uses default)
//   - tableName: table name to look up
//   - normalizer: function to normalize identifiers for comparison
//
// Returns:
//   - Column slice, or nil if table not found
func GetColumnsForTable(
	meta Metadata,
	schemaName, tableName string,
	normalizer func(string) string,
) []Column {
	if schemaName == "" {
		schemaName = GetDefaultSchema(meta)
	}

	normalizedSchema := normalizer(schemaName)
	normalizedTable := normalizer(tableName)

	for _, schema := range meta.Schemas {
		if normalizer(schema.Name) != normalizedSchema {
			continue
		}

		for _, table := range GetAllTablesFromSchema(schema) {
			if normalizer(table.Name) == normalizedTable {
				return table.Columns
			}
		}
	}

	return nil
}

// IsKeywordTokenType checks if a token is one of the given keyword types
// ============================================
// IDENTIFIER CHARACTER HELPERS
// ============================================

// IsIdentifierChar checks if a character can be part of a SQL identifier
func IsIdentifierChar(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}

// ExtractFirstIdentifier extracts the first identifier-like token from text
// Returns the identifier and any table prefix if found (for qualified names)
func ExtractFirstIdentifier(text string, quoteChar rune, normalizer func(string) string) (identifier, tablePrefix string) {
	// Check for qualified name (table.column)
	if parts := SplitQualifiedIdentifier(text, quoteChar); len(parts) > 1 {
		return normalizer(parts[len(parts)-1]), normalizer(parts[len(parts)-2])
	}

	// Simple expression - extract first identifier-like token
	var colName string
	for _, ch := range text {
		if IsIdentifierChar(ch) || (colName != "" && ch >= '0' && ch <= '9') {
			colName += string(ch)
		} else if colName != "" {
			break
		}
	}

	return normalizer(colName), ""
}

// ContainsIdentifier checks if text contains an identifier as a separate token
func ContainsIdentifier(text, identifier string, normalizer func(string) string) bool {
	lowerText := normalizer(text)
	lowerIdent := normalizer(identifier)

	for idx := 0; idx <= len(lowerText)-len(lowerIdent); idx++ {
		if lowerText[idx:idx+len(lowerIdent)] == lowerIdent {
			validBefore := idx == 0 || !IsIdentifierChar(rune(lowerText[idx-1]))
			validAfter := idx+len(lowerIdent) == len(lowerText) || !IsIdentifierChar(rune(lowerText[idx+len(lowerIdent)]))
			if validBefore && validAfter {
				return true
			}
		}
	}
	return false
}

// ============================================
// INSPECT RESULT HELPERS
// ============================================

// MergeInspectTables merges tables into a slice, avoiding duplicates
func MergeInspectTables(tables []InspectTable, newTables []InspectTable) []InspectTable {
	for _, newTable := range newTables {
		found := false
		for _, t := range tables {
			if t.Name == newTable.Name && t.Schema == newTable.Schema {
				found = true
				break
			}
		}
		if !found {
			tables = append(tables, newTable)
		}
	}
	return tables
}

// MergeInspectFields merges fields into a slice, avoiding duplicates by (name, table, schema)
func MergeInspectFields(fields []InspectField, newFields []InspectField) []InspectField {
	for _, newField := range newFields {
		found := false
		for _, f := range fields {
			if f.Name == newField.Name && f.Table == newField.Table && f.Schema == newField.Schema {
				found = true
				break
			}
		}
		if !found {
			fields = append(fields, newField)
		}
	}
	return fields
}

// GetColumnsForTableAsColumns gets full column info for a table using dialect-agnostic methods
// This is a dialect-agnostic utility that can be used by any SQL dialect
func GetColumnsForTableAsColumns(meta Metadata, schemaName, tableName string, dialect SQLDialect) []Column {
	if schemaName == "" {
		schemaName = GetDefaultSchema(meta)
	}
	normalizedSchema := dialect.NormalizeIdentifier(schemaName)
	normalizedTable := dialect.NormalizeIdentifier(tableName)

	for _, schema := range meta.Schemas {
		if dialect.NormalizeIdentifier(schema.Name) != normalizedSchema {
			continue
		}
		for _, table := range GetAllTablesFromSchema(schema) {
			if dialect.NormalizeIdentifier(table.Name) == normalizedTable {
				return table.Columns
			}
		}
	}
	return nil
}

// TableExistsInMetadata reports whether a table is known in the metadata.
func TableExistsInMetadata(meta Metadata, schemaName, tableName string, dialect SQLDialect) bool {
	return GetColumnsForTableAsColumns(meta, schemaName, tableName, dialect) != nil
}

// ResolveColumnFromRelationRefs resolves a column against FROM-clause references using an optional
// qualifier chain: no qualifiers means an unqualified column name; one qualifier matches table
// alias or table name; two qualifiers match schema and table (empty ref schema is treated as
// the metadata default schema).
func ResolveColumnFromRelationRefs(
	qualifiers []string,
	columnName string,
	refs []RelationRef,
	meta Metadata,
	dialect SQLDialect,
) []Column {
	norm := func(s string) string { return dialect.NormalizeIdentifier(s) }
	colNorm := norm(columnName)
	defSch := norm(GetDefaultSchema(meta))

	appendUnknown := func() []Column {
		return []Column{{Name: colNorm, Type: "unknown", Nullable: true}}
	}

	switch len(qualifiers) {
	case 0:
		for _, ref := range refs {
			for _, c := range GetColumnsForTableAsColumns(meta, ref.Schema, ref.Table, dialect) {
				if norm(c.Name) == colNorm {
					return []Column{c}
				}
			}
		}
		return appendUnknown()
	case 1:
		q := norm(qualifiers[0])
		for _, ref := range refs {
			key := ref.Alias
			if key == "" {
				key = ref.Table
			}
			if norm(key) != q {
				continue
			}
			for _, c := range GetColumnsForTableAsColumns(meta, ref.Schema, ref.Table, dialect) {
				if norm(c.Name) == colNorm {
					return []Column{c}
				}
			}
		}
		return appendUnknown()
	case 2:
		schQ, tblQ := norm(qualifiers[0]), norm(qualifiers[1])
		for _, ref := range refs {
			refSch := norm(ref.Schema)
			if refSch == "" {
				refSch = defSch
			}
			if refSch != schQ || norm(ref.Table) != tblQ {
				continue
			}
			for _, c := range GetColumnsForTableAsColumns(meta, ref.Schema, ref.Table, dialect) {
				if norm(c.Name) == colNorm {
					return []Column{c}
				}
			}
		}
		return appendUnknown()
	default:
		return appendUnknown()
	}
}

// ScopeContainsCaret reports whether caretPos falls within the token-index scope [startPos, endPos].
//
// This is the single canonical implementation of the scope boundary check shared by:
//   - ref_resolve.go: refActiveAtCaret / vtActiveAtCaret
//   - completion.go:  CompletionStrategy.isInScope
//
// Sentinel conventions (used by both callers):
//   - startPos == -1  → scope tracking was not recorded; treat as always in scope.
//   - startPos == 0   → no start boundary (0 is used as a placeholder in tests).
//   - endPos <= 0     → no end boundary (open-ended scope).
func ScopeContainsCaret(startPos, endPos, caretPos int) bool {
	if startPos == -1 {
		return true // no scope tracking available, treat as always in scope
	}
	if startPos > 0 && caretPos < startPos {
		return false
	}
	if endPos > 0 && caretPos > endPos {
		return false
	}
	return true
}
