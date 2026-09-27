package core

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// SchemaRows is a whole database schema as flat rows. A dialect reads it with
// one UNION ALL query through QuerySchemaRows, then calls Metadata.
type SchemaRows struct {
	Schemas      []SchemaRow
	Current      string
	Relations    []RelationRow
	Columns      []ColumnRow
	PrimaryKeys  []KeyColumnRow
	ForeignKeys  []ForeignKeyRow
	Indexes      []IndexRow
	IndexColumns []IndexColumnRow
	Triggers     []TriggerRow
	Stats        []StatRow
	Types        []Type
	Functions    []Function
	Settings     []Setting
	// Other holds rows of kinds QuerySchemaRows does not know, for the dialect.
	Other map[string][]json.RawMessage
}

// SchemaRow names a schema. A catalog schema holds the built-in objects and
// the settings, and is listed after the user schemas.
type SchemaRow struct {
	Name    string `json:"name"`
	Catalog Flag   `json:"catalog"`
}

// RelationRow is a table or a view; Kind is "table" or "view".
type RelationRow struct {
	Schema      string `json:"schema"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	DDL         string `json:"ddl"`
	Description string `json:"description"`
}

type ColumnRow struct {
	Schema      string         `json:"schema"`
	Table       string         `json:"table"`
	Name        string         `json:"name"`
	Type        string         `json:"type"`
	Nullable    Flag           `json:"nullable"`
	Default     *string        `json:"default"`
	Position    int            `json:"position"`
	Description string         `json:"description"`
	Extra       map[string]any `json:"extra"`
}

// KeyColumnRow is one column of a table's primary key.
type KeyColumnRow struct {
	Schema   string `json:"schema"`
	Table    string `json:"table"`
	Column   string `json:"column"`
	Position int    `json:"position"`
}

type ForeignKeyRow struct {
	Schema    string `json:"schema"`
	Table     string `json:"table"`
	Column    string `json:"column"`
	RefSchema string `json:"refSchema"`
	RefTable  string `json:"refTable"`
	RefColumn string `json:"refColumn"`
}

type IndexRow struct {
	Schema string `json:"schema"`
	Table  string `json:"table"`
	Name   string `json:"name"`
	DDL    string `json:"ddl"`
}

// IndexColumnRow is one column of an index. An expression has no Name and is
// left out of the index's columns.
type IndexColumnRow struct {
	Schema     string `json:"schema"`
	Table      string `json:"table"`
	Index      string `json:"index"`
	Name       string `json:"name"`
	Position   int    `json:"position"`
	Collation  string `json:"collation"`
	Descending Flag   `json:"descending"`
}

type TriggerRow struct {
	Schema string `json:"schema"`
	Table  string `json:"table"`
	Name   string `json:"name"`
	DDL    string `json:"ddl"`
}

// StatRow is one table or index statistic.
type StatRow struct {
	Schema string `json:"schema"`
	Name   string `json:"name"`
	Value  string `json:"value"`
}

// Flag is a JSON boolean that also reads 1 and 0, which is what MySQL's
// JSON_OBJECT writes for a comparison.
type Flag bool

func (f *Flag) UnmarshalJSON(b []byte) error {
	switch string(b) {
	case "true", "1":
		*f = true
	case "false", "0", "null":
		*f = false
	default:
		return fmt.Errorf("not a boolean: %s", b)
	}
	return nil
}

// QuerySchemaRows runs a schema query whose rows are (kind, JSON object).
func QuerySchemaRows(ctx context.Context, db *sql.DB, query string) (SchemaRows, error) {
	var r SchemaRows
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return r, fmt.Errorf("failed to read schema: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var kind string
		var doc sql.RawBytes
		if err := rows.Scan(&kind, &doc); err != nil {
			return r, fmt.Errorf("failed to scan schema row: %w", err)
		}
		if err := decodeRow(&r, kind, doc); err != nil {
			return r, fmt.Errorf("failed to decode %s row: %w", kind, err)
		}
	}
	return r, rows.Err()
}

func decodeRow(r *SchemaRows, kind string, doc []byte) error {
	switch kind {
	case "schema":
		return appendRow(doc, &r.Schemas)
	case "current":
		var c struct {
			Name string `json:"name"`
		}
		err := json.Unmarshal(doc, &c)
		r.Current = c.Name
		return err
	case "relation":
		return appendRow(doc, &r.Relations)
	case "column":
		return appendRow(doc, &r.Columns)
	case "primary_key":
		return appendRow(doc, &r.PrimaryKeys)
	case "foreign_key":
		return appendRow(doc, &r.ForeignKeys)
	case "index":
		return appendRow(doc, &r.Indexes)
	case "index_column":
		return appendRow(doc, &r.IndexColumns)
	case "trigger":
		return appendRow(doc, &r.Triggers)
	case "stat":
		return appendRow(doc, &r.Stats)
	case "type":
		return appendRow(doc, &r.Types)
	case "function":
		return appendRow(doc, &r.Functions)
	case "setting":
		return appendRow(doc, &r.Settings)
	}
	if r.Other == nil {
		r.Other = make(map[string][]json.RawMessage)
	}
	// doc is only valid until the next row: RawBytes is not copied on scan.
	r.Other[kind] = append(r.Other[kind], append(json.RawMessage(nil), doc...))
	return nil
}

// DecodeOther decodes the rows of a kind this package does not know into dst,
// a pointer to a slice of the row type.
func (r SchemaRows) DecodeOther(kind string, dst any) error {
	doc := []byte{'['}
	for i, row := range r.Other[kind] {
		if i > 0 {
			doc = append(doc, ',')
		}
		doc = append(doc, row...)
	}
	if err := json.Unmarshal(append(doc, ']'), dst); err != nil {
		return fmt.Errorf("failed to decode %s row: %w", kind, err)
	}
	return nil
}

func appendRow[T any](doc []byte, dst *[]T) error {
	var v T
	if err := json.Unmarshal(doc, &v); err != nil {
		return err
	}
	*dst = append(*dst, v)
	return nil
}

// compareNames orders names the way a case-insensitive collation would, so
// the result does not depend on the order the database returned rows in.
func compareNames(a, b string) int {
	return cmp.Or(strings.Compare(strings.ToLower(a), strings.ToLower(b)), strings.Compare(a, b))
}

// relation names a table or view within its schema.
type relation struct{ schema, name string }

// Metadata groups the rows into schemas: user schemas by name, then catalogs.
// Tables get their primary and foreign keys; views keep their columns as read.
func (r SchemaRows) Metadata() *Metadata {
	slices.SortStableFunc(r.Schemas, func(a, b SchemaRow) int {
		if a.Catalog != b.Catalog {
			if a.Catalog {
				return 1
			}
			return -1
		}
		return compareNames(a.Name, b.Name)
	})
	slices.SortStableFunc(r.Relations, func(a, b RelationRow) int {
		return cmp.Or(compareNames(a.Schema, b.Schema), compareNames(a.Name, b.Name))
	})
	slices.SortStableFunc(r.Columns, func(a, b ColumnRow) int { return cmp.Compare(a.Position, b.Position) })
	slices.SortStableFunc(r.PrimaryKeys, func(a, b KeyColumnRow) int { return cmp.Compare(a.Position, b.Position) })
	slices.SortStableFunc(r.Indexes, func(a, b IndexRow) int {
		return cmp.Or(compareNames(a.Table, b.Table), compareNames(a.Name, b.Name))
	})
	slices.SortStableFunc(r.IndexColumns, func(a, b IndexColumnRow) int { return cmp.Compare(a.Position, b.Position) })
	slices.SortStableFunc(r.Triggers, func(a, b TriggerRow) int { return compareNames(a.Name, b.Name) })
	slices.SortStableFunc(r.Types, func(a, b Type) int { return compareNames(a.Name, b.Name) })
	slices.SortStableFunc(r.Functions, func(a, b Function) int {
		return cmp.Or(compareNames(a.Name, b.Name), cmp.Compare(a.OID, b.OID))
	})
	slices.SortStableFunc(r.Settings, func(a, b Setting) int { return compareNames(a.Name, b.Name) })

	meta := &Metadata{CurrentSchema: r.Current, Schemas: make([]Schema, len(r.Schemas))}
	bySchema := make(map[string]*Schema, len(r.Schemas))
	for i, s := range r.Schemas {
		meta.Schemas[i].Name = s.Name
		if s.Catalog {
			meta.Schemas[i].Settings = r.Settings
		} else {
			meta.Schemas[i].ForeignTables = []Table{}
			meta.Schemas[i].MaterializedViews = []Table{}
		}
		bySchema[s.Name] = &meta.Schemas[i]
	}

	columns := make(map[relation][]Column)
	for _, c := range r.Columns {
		k := relation{c.Schema, c.Table}
		columns[k] = append(columns[k], Column{
			Name: c.Name, Type: c.Type, Nullable: bool(c.Nullable), Default: c.Default,
			Description: c.Description, Extra: c.Extra,
		})
	}
	primaryKeys := make(map[relation][]string)
	for _, pk := range r.PrimaryKeys {
		k := relation{pk.Schema, pk.Table}
		primaryKeys[k] = append(primaryKeys[k], pk.Column)
	}
	foreignKeys := make(map[relation]map[string]ForeignKeyRef)
	for _, fk := range r.ForeignKeys {
		k := relation{fk.Schema, fk.Table}
		if foreignKeys[k] == nil {
			foreignKeys[k] = make(map[string]ForeignKeyRef)
		}
		foreignKeys[k][fk.Column] = ForeignKeyRef{SchemaName: fk.RefSchema, TableName: fk.RefTable, ColumnName: fk.RefColumn}
	}
	for _, rel := range r.Relations {
		s := bySchema[rel.Schema]
		if s == nil {
			continue
		}
		k := relation{rel.Schema, rel.Name}
		t := Table{Name: rel.Name, Columns: columns[k], DDL: rel.DDL, Description: rel.Description}
		if rel.Kind == "view" {
			s.Views = append(s.Views, t)
			continue
		}
		t.PrimaryKey = primaryKeys[k]
		EnrichColumnsWithConstraints(&t.Columns, t.PrimaryKey, foreignKeys[k])
		s.Tables = append(s.Tables, t)
	}

	indexColumns := make(map[[3]string][]IndexColumnInfo)
	for _, c := range r.IndexColumns {
		if c.Name == "" {
			continue
		}
		k := [3]string{c.Schema, c.Table, c.Index}
		indexColumns[k] = append(indexColumns[k], IndexColumnInfo{
			Name: c.Name, Position: c.Position, Collation: c.Collation, Descending: bool(c.Descending),
		})
	}
	for _, idx := range r.Indexes {
		if s := bySchema[idx.Schema]; s != nil {
			s.Indexes = append(s.Indexes, IndexInfo{
				Name: idx.Name, TableName: idx.Table, DDL: idx.DDL,
				Columns: indexColumns[[3]string{idx.Schema, idx.Table, idx.Name}],
			})
		}
	}
	for _, tg := range r.Triggers {
		if s := bySchema[tg.Schema]; s != nil {
			s.Triggers = append(s.Triggers, TriggerInfo{Name: tg.Name, TableName: tg.Table, DDL: tg.DDL})
		}
	}
	for _, st := range r.Stats {
		if s := bySchema[st.Schema]; s != nil {
			if s.Stats == nil {
				s.Stats = make(TableStats)
			}
			s.Stats[st.Name] = st.Value
		}
	}
	for _, t := range r.Types {
		if s := bySchema[t.Schema]; s != nil {
			s.Types = append(s.Types, t)
		}
	}
	for _, f := range r.Functions {
		if s := bySchema[f.Schema]; s != nil {
			s.Functions = append(s.Functions, f)
		}
	}
	return meta
}
