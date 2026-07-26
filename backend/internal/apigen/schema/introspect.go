package schema

import (
	"context"
	"database/sql"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/postgresql"
)

// Introspect reads the given schemas through the dialect's Postgres
// introspection — the same engine the SQL IDE uses — and maps the result into
// RawSchema. Table and column COMMENTs (now captured by the dialect) carry the
// @app tags; PK/FK/types come enriched from GetTables.
func Introspect(ctx context.Context, db *sql.DB, schemas ...string) (RawSchema, error) {
	d := postgresql.NewDialect()
	var out RawSchema
	for _, schema := range schemas {
		tables, err := d.GetTables(ctx, db, schema)
		if err != nil {
			return RawSchema{}, err
		}
		for _, t := range tables {
			out.Tables = append(out.Tables, mapTable(schema, t))
		}
	}
	return out, nil
}

func mapTable(schema string, t core.Table) RawTable {
	rt := RawTable{Schema: schema, Name: t.Name, Comment: t.Description, PrimaryKey: t.PrimaryKey}
	for _, c := range t.Columns {
		rc := RawColumn{
			Name:     c.Name,
			DataType: c.Type,
			NotNull:  !c.Nullable,
			Comment:  c.Description,
		}
		if c.Default != nil {
			rc.Default = *c.Default
		}
		rt.Columns = append(rt.Columns, rc)
		if c.IsForeignKey && c.ForeignKey != nil {
			rt.ForeignKeys = append(rt.ForeignKeys, RawFK{
				Column:    c.Name,
				RefSchema: c.ForeignKey.SchemaName,
				RefTable:  c.ForeignKey.TableName,
				RefColumn: c.ForeignKey.ColumnName,
			})
		}
	}
	return rt
}
