package schema

import (
	"context"
	"database/sql"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/postgresql"
)

// Introspect reads the given schemas through the dialect's Postgres
// introspection, the same engine the SQL IDE uses, and maps the result into
// RawSchema. Table and column COMMENTs (now captured by the dialect) carry the
// @app tags; PK/FK/types come enriched from ReadSchema.
func Introspect(ctx context.Context, db *sql.DB, schemas ...string) (RawSchema, error) {
	meta, err := postgresql.NewDialect().ReadSchema(ctx, db)
	if err != nil {
		return RawSchema{}, err
	}
	byName := make(map[string]core.Schema, len(meta.Schemas))
	for _, s := range meta.Schemas {
		byName[s.Name] = s
	}
	var out RawSchema
	for _, name := range schemas {
		for _, t := range byName[name].Tables {
			out.Tables = append(out.Tables, mapTable(name, t))
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
