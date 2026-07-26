package syncer

import (
	"backend/internal/apigen/codegen"
	"backend/internal/apigen/schema"
	"fmt"
	"strings"
	"text/template"
)

// EmitSyncSQL renders the four sqlc CRUD queries every synced entity needs:
// by-id (workspace-scoped), changes-since (workspace-membership scoped via
// workspace_to_user), soft-delete, and upsert. Uniform across all tables —
// `workspace` is the one self-tenant exception and stays hand-written.
func EmitSyncSQL(e schema.Entity) []codegen.GenFile {
	sing := codegen.Pascal(e.Table) // Role
	d := sqlData{
		Sing:       sing,
		Plur:       sing + "s", // Roles (naive; irregular plurals get an override later)
		Table:      e.Schema + "." + e.Table,
		PK:         e.PrimaryKey[0],
		Tenant:     schema.TenantColumn,
		Cursor:     schema.CursorColumn,
		SoftDelete: schema.SoftDeleteColumn,
	}

	var cols, rcols []string
	for _, f := range e.Fields {
		cols = append(cols, f.Column)
		rcols = append(rcols, "r."+f.Column)
	}
	d.Cols = strings.Join(cols, ", ")
	d.RCols = strings.Join(rcols, ", ")

	// INSERT every column except the soft-delete, in column order; the cursor is
	// now(), the rest bind a param. ON CONFLICT updates only the patchable
	// (non-FK, non-system) columns — FKs and the tenant are identity, set once.
	var insCols, vals []string
	n := 0
	for _, f := range e.Fields {
		if f.Column == schema.SoftDeleteColumn {
			continue
		}
		insCols = append(insCols, f.Column)
		if f.Column == schema.CursorColumn {
			vals = append(vals, "now()")
		} else {
			n++
			vals = append(vals, fmt.Sprintf("$%d", n))
		}
	}
	d.InsCols = strings.Join(insCols, ", ")
	d.Vals = strings.Join(vals, ", ")

	var setLines []string
	for _, f := range e.Fields {
		if f.Patchable {
			setLines = append(setLines, fmt.Sprintf("  %s = EXCLUDED.%s,", f.Column, f.Column))
		}
	}
	setLines = append(setLines, fmt.Sprintf("  %s = now(),", schema.CursorColumn))
	setLines = append(setLines, fmt.Sprintf("  %s = NULL;", schema.SoftDeleteColumn))
	d.SetBlock = strings.Join(setLines, "\n")

	return []codegen.GenFile{
		{Name: fmt.Sprintf("get_%s_by_id_query.sql", e.Table), Content: renderSQL(sqlByIDTmpl, d)},
		{Name: fmt.Sprintf("get_%ss_for_user_since_query.sql", e.Table), Content: renderSQL(sqlSinceTmpl, d)},
		{Name: fmt.Sprintf("set_%s_deleted_at_statement.sql", e.Table), Content: renderSQL(sqlDeleteTmpl, d)},
		{Name: fmt.Sprintf("upsert_%s_statement.sql", e.Table), Content: renderSQL(sqlUpsertTmpl, d)},
	}
}

// sqlData is the view a SQL template renders. List columns are pre-joined so the
// templates stay pure substitution (no ranges), which keeps their whitespace —
// significant, since SQL output isn't run through a formatter — trivial to read.
type sqlData struct {
	Sing, Plur                 string
	Table, PK, Tenant, Cursor  string
	SoftDelete                 string
	Cols, RCols, InsCols, Vals string
	SetBlock                   string
}

func renderSQL(t *template.Template, d sqlData) string {
	var b strings.Builder
	if err := t.Execute(&b, d); err != nil {
		panic(err) // templates are constant; a failure is a programming error
	}
	return b.String()
}

var (
	sqlByIDTmpl = template.Must(template.New("byid").Parse(
		"-- name: Get{{.Sing}}ByID :one\n" +
			"SELECT {{.Cols}}\n" +
			"FROM {{.Table}}\n" +
			"WHERE {{.PK}} = $1 AND {{.Tenant}} = $2;\n"))

	sqlSinceTmpl = template.Must(template.New("since").Parse(
		"-- name: Get{{.Plur}}ForUserSince :many\n" +
			"SELECT {{.RCols}}\n" +
			"FROM {{.Table}} r\n" +
			"INNER JOIN app.workspace_to_user wtu ON wtu.workspace_id = r.{{.Tenant}} AND wtu.user_id = $1\n" +
			"WHERE r.{{.Cursor}} > $2;\n"))

	sqlDeleteTmpl = template.Must(template.New("delete").Parse(
		"-- name: Set{{.Sing}}DeletedAt :exec\n" +
			"UPDATE {{.Table}}\n" +
			"SET {{.SoftDelete}} = now(), {{.Cursor}} = now()\n" +
			"WHERE {{.PK}} = $1 AND {{.Tenant}} = $2;\n"))

	sqlUpsertTmpl = template.Must(template.New("upsert").Parse(
		"-- name: Upsert{{.Sing}} :exec\n" +
			"INSERT INTO {{.Table}} ({{.InsCols}})\n" +
			"VALUES ({{.Vals}})\n" +
			"ON CONFLICT ({{.PK}}) DO UPDATE SET\n" +
			"{{.SetBlock}}\n"))
)
