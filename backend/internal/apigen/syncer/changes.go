package syncer

import (
	"backend/internal/apigen/codegen"
	"backend/internal/apigen/schema"
	"fmt"
	"go/format"
	"sort"
	"strings"
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

var changesTmpl = mustParse("changes.go.tmpl")
