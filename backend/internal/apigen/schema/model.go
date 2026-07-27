// Package schema builds the schema-driven entity model — the IR every apigen
// projection consumes — from the SQL catalog plus a thin @app.* comment-tag
// layer. It holds the model types, the tag parser, the catalog introspector,
// and the derivation (catalog + convention + tags -> Entity).
package schema

import (
	"fmt"
	"strings"
)

// Kind is the logical column type, derived from the SQL type.
type Kind string

const (
	KindText Kind = "text"
	KindUUID Kind = "uuid"
	KindInt  Kind = "int"
	KindBool Kind = "bool"
	KindTime Kind = "time"
	KindInet Kind = "inet"
	KindJSON Kind = "json"
)

// Convention columns every @app.sync table carries. lint enforces their
// presence, so the emitters assume them by name rather than deriving a
// per-entity column for each — a table lacking any of them can't be generated.
const (
	TenantColumn     = "workspace_id" // FK to workspace — the tenant scope
	CursorColumn     = "updated_at"   // last-write-wins cursor
	SoftDeleteColumn = "deleted_at"   // soft-delete marker
	IDColumn         = "id"           // surrogate primary key
)

func kindOf(sqlType string) Kind {
	t := strings.ToLower(sqlType)
	switch {
	case strings.Contains(t, "uuid"):
		return KindUUID
	case strings.HasPrefix(t, "bool"):
		return KindBool
	case strings.Contains(t, "timestamp"), strings.HasPrefix(t, "date"), strings.Contains(t, "time"):
		return KindTime
	case strings.Contains(t, "json"):
		return KindJSON
	case strings.HasPrefix(t, "inet"):
		return KindInet
	case strings.Contains(t, "int"):
		return KindInt
	default:
		return KindText
	}
}

// Raw* mirror the catalog before derivation; Introspect fills them.
type RawColumn struct {
	Name     string
	DataType string
	NotNull  bool
	Comment  string
	Default  string // raw catalog default expression, e.g. 'local'::text
}

type RawFK struct {
	Column    string
	RefSchema string
	RefTable  string
	RefColumn string
}

type RawTable struct {
	Schema      string
	Name        string
	Comment     string
	Columns     []RawColumn
	PrimaryKey  []string
	ForeignKeys []RawFK
}

type RawSchema struct{ Tables []RawTable }

// FKRef is the target of a foreign key.
type FKRef struct{ Schema, Table, Column string }

// Field is one column plus its derived + declared metadata.
type Field struct {
	Name      string
	Column    string
	Kind      Kind
	Nullable  bool
	IsPK      bool
	FK        *FKRef `json:",omitempty"`
	Patchable bool
	// Exposed is false for the tenant and soft-delete columns (always enforced
	// server-side, never selectable/filterable) and for @app.hide columns.
	Exposed bool
	// Filterable is Exposed minus the technical columns (cursor, surrogate id)
	// that shouldn't be offered as query filters. Business columns and FKs stay
	// filterable, as do meaningful columns that happen to be in a composite PK.
	Filterable bool
	Hidden     bool     `json:",omitempty"`
	Values    []string `json:",omitempty"`
	JSONPaths []string `json:",omitempty"`
	Ops       []string `json:",omitempty"`
	Lookup    string   `json:",omitempty"`
	Immutable bool     `json:",omitempty"`
	// Default is the literal value of a simple string-literal column default
	// (e.g. 'local'), used to coalesce a NOT NULL column the client omits.
	// Empty for no default or a non-literal (function) default.
	Default string `json:",omitempty"`
}

// Relation is a named, possibly unioned join path to a target table.
type Relation struct {
	Name   string
	Target string // last hop's table
	Paths  [][]Edge
}

// APIOp is one enrolled endpoint op with its (AND-list) required actions.
type APIOp struct {
	Op       string
	Requires []string `json:",omitempty"`
}

// Entity is the derived model for one table.
type Entity struct {
	Name       string
	Schema     string
	Table      string
	Sync       bool
	PrimaryKey []string
	ScopeVia   string     `json:",omitempty"`
	API        []APIOp    `json:",omitempty"`
	Relations  []Relation `json:",omitempty"`
	Fields     []Field
}

// Build derives the entity model from the raw catalog + parsed @app tags.
// It returns every entity plus any lint errors (which don't abort the rest).
func Build(s RawSchema) ([]Entity, []error) {
	var ents []Entity
	var errs []error
	for _, t := range s.Tables {
		tags := ParseTags(t.Comment)
		e := Entity{
			Name:       or(tags.Entity, t.Name),
			Schema:     t.Schema,
			Table:      t.Name,
			Sync:       tags.Sync,
			PrimaryKey: t.PrimaryKey,
			ScopeVia:   tags.Scope,
			API:        tags.API,
			Relations:  buildRelations(tags.Relations),
		}

		pk := set(t.PrimaryKey)
		fkByCol := map[string]RawFK{}
		for _, fk := range t.ForeignKeys {
			fkByCol[fk.Column] = fk
		}

		for _, c := range t.Columns {
			ct := ParseColumnTags(c.Comment)
			f := Field{
				Name: c.Name, Column: c.Name, Kind: kindOf(c.DataType), Nullable: !c.NotNull,
				IsPK: pk[c.Name], Hidden: ct.Hide, Values: ct.Values, JSONPaths: ct.JSONPaths,
				Ops: ct.Ops, Lookup: ct.Lookup, Immutable: ct.Immutable,
				Default: parseDefaultLiteral(c.Default),
			}
			if fk, ok := fkByCol[c.Name]; ok {
				f.FK = &FKRef{Schema: fk.RefSchema, Table: fk.RefTable, Column: fk.RefColumn}
			}
			// Patchable = the row's own value columns: not PK, not tenant/cursor/
			// soft-delete, and not a foreign key (FKs are relationship identity,
			// passed through on insert but never updated on conflict).
			f.Patchable = !f.IsPK && !f.Immutable && f.FK == nil &&
				c.Name != TenantColumn && c.Name != CursorColumn && c.Name != SoftDeleteColumn
			// Tenant + soft-delete are enforced by the engine (workspace_id =
			// caller, deleted_at IS NULL), so they are never client-visible.
			f.Exposed = !f.Hidden && c.Name != TenantColumn && c.Name != SoftDeleteColumn
			// Filterable drops the technical columns (cursor, surrogate id) that
			// are exposed but shouldn't be query filters.
			f.Filterable = f.Exposed && c.Name != CursorColumn && c.Name != IDColumn
			e.Fields = append(e.Fields, f)
		}

		errs = append(errs, lint(e)...)
		ents = append(ents, e)
	}
	return ents, errs
}

// buildRelations groups declarations by name; repeats become union paths.
func buildRelations(decls []RelationDecl) []Relation {
	var order []string
	byName := map[string]*Relation{}
	for _, d := range decls {
		r := byName[d.Name]
		if r == nil {
			r = &Relation{Name: d.Name}
			byName[d.Name] = r
			order = append(order, d.Name)
		}
		r.Paths = append(r.Paths, d.Path)
		r.Target = d.Path[len(d.Path)-1].ToTable
	}
	var out []Relation
	for _, n := range order {
		out = append(out, *byName[n])
	}
	return out
}

// lint holds the step-0 assertions; more land as emitters are added. A synced
// table must have a primary key and carry every convention column the emitters
// assume — a missing one means the table simply can't be generated.
func lint(e Entity) []error {
	if !e.Sync {
		return nil
	}
	var errs []error
	if len(e.PrimaryKey) == 0 {
		errs = append(errs, fmt.Errorf("%s: @app.sync but no primary key", e.Name))
	}
	for _, col := range []string{TenantColumn, CursorColumn, SoftDeleteColumn} {
		if !hasColumn(e, col) {
			errs = append(errs, fmt.Errorf("%s: @app.sync but missing convention column %q", e.Name, col))
		}
	}
	return errs
}

func hasColumn(e Entity, name string) bool {
	for _, f := range e.Fields {
		if f.Column == name {
			return true
		}
	}
	return false
}

// parseDefaultLiteral extracts the literal value of a simple string-literal
// column default, stripping a type cast and the surrounding quotes:
// 'local'::text -> local. Function/expression defaults (now(),
// gen_random_uuid(), nextval(...)) have no usable literal and return "".
func parseDefaultLiteral(expr string) string {
	s := strings.TrimSpace(expr)
	if i := strings.LastIndex(s, "::"); i >= 0 { // drop trailing ::type cast
		s = strings.TrimSpace(s[:i])
	}
	if len(s) < 2 || s[0] != '\'' || s[len(s)-1] != '\'' {
		return "" // not a plain quoted literal (e.g. a function default)
	}
	return strings.ReplaceAll(s[1:len(s)-1], "''", "'")
}

// FilterOperators is the filter operator set a field supports, derived from its
// kind. An explicit @app.ops list overrides it wholesale. Shared by every
// projection (capabilities, OpenAPI) so the operator taxonomy stays single-
// sourced.
func FilterOperators(f Field) []string {
	if len(f.Ops) > 0 {
		return f.Ops
	}
	null := []string{"is_null", "not_null"}
	switch f.Kind {
	case KindText:
		return append([]string{"eq", "ne", "in", "nin", "like", "ilike"}, null...)
	case KindUUID, KindInet:
		return append([]string{"eq", "ne", "in", "nin"}, null...)
	case KindInt, KindTime:
		return append([]string{"eq", "ne", "lt", "lte", "gt", "gte", "in", "nin"}, null...)
	case KindBool:
		return append([]string{"eq", "ne"}, null...)
	case KindJSON:
		return append([]string{"contains"}, null...)
	default:
		return null
	}
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func set(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}
