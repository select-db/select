package syncer

import (
	"backend/internal/apigen/codegen"
	"backend/internal/apigen/schema"
	"fmt"
	"go/format"
	"sort"
	"strings"
	"text/template"
)

// changeTable is one @app.sync table's projection onto SyncChanges: the struct
// field it fills, the package whose GetChangesSince produces it, and the row
// type FilterByID closes over.
type changeTable struct{ Field, Pkg, RowType string }

// EmitSyncChanges renders changes.go: the aggregation that fills every
// @app.sync field of types.SyncChanges (FetchChanges), the applied-id filter
// over those same fields (FilterApplied), and the generic FilterByID they and
// the hand-written syncer share. The hand-written syncer composes these with
// the specials — the workspace field and the related-users side channel — and
// owns the SyncChanges struct itself.
func EmitSyncChanges(entities []schema.Entity) (codegen.GenFile, error) {
	var tables []changeTable
	for _, e := range entities {
		if !e.Sync {
			continue
		}
		sing := codegen.Pascal(e.Table)
		tables = append(tables, changeTable{Field: sing, Pkg: e.Table, RowType: "types." + sing + "Row"})
	}
	sort.Slice(tables, func(i, j int) bool { return tables[i].Pkg < tables[j].Pkg })

	var b strings.Builder
	if err := changesTmpl.Execute(&b, tables); err != nil {
		return codegen.GenFile{}, err
	}
	formatted, err := format.Source([]byte(b.String()))
	if err != nil {
		return codegen.GenFile{}, fmt.Errorf("changes.go: %w", err)
	}
	return codegen.GenFile{Name: "changes.go", Content: string(formatted)}, nil
}

var changesTmpl = template.Must(template.New("changes").Parse(genHeader +
	`package gen

import (
	"context"
	"time"

	"backend/internal/syncer/types"
{{- range .}}
	"backend/internal/syncer/gen/{{.Pkg}}"
{{- end}}
)

// FetchChanges fills the @app.sync fields of changes with every row updated for
// userID since ` + "`since`" + `. The specials (the workspace field and the
// related-users side channel) are composed by the hand-written getChangesSince.
func FetchChanges(ctx context.Context, userID string, since time.Time, changes *types.SyncChanges) error {
	var err error
{{- range .}}
	if changes.{{.Field}}, err = {{.Pkg}}.GetChangesSince(ctx, userID, since); err != nil {
		return err
	}
{{- end}}
	return nil
}

// FilterApplied drops rows from the @app.sync fields whose id was applied in
// this request, so a client isn't echoed its own writes back.
func FilterApplied(changes *types.SyncChanges, applied map[string]struct{}) {
{{- range .}}
	changes.{{.Field}} = FilterByID(changes.{{.Field}}, applied, func(r {{.RowType}}) string { return r.ID })
{{- end}}
}

// FilterByID removes rows whose ID is in the applied set. The hand-written
// syncer uses it for the workspace special too.
func FilterByID[T any](rows []T, applied map[string]struct{}, id func(T) string) []T {
	filtered := rows[:0]
	for _, r := range rows {
		if _, ok := applied[id(r)]; !ok {
			filtered = append(filtered, r)
		}
	}
	return filtered
}
`))
