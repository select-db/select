package syncer

import (
	"backend/internal/apigen/codegen"
	"backend/internal/apigen/schema"
	"fmt"
	"go/format"
	"sort"
	"strings"
)

// EmitSyncDispatch renders the dispatch registry (package gen) mapping each
// @app.sync table to its Apply / ApplyDelete / FetchCurrent. The hand-written
// syncer composes this with the specials (workspace) and owns the rest of the
// orchestration (token refresh, related-user resolution, changes aggregation).
func EmitSyncDispatch(entities []schema.Entity) (codegen.GenFile, error) {
	var tables []string
	for _, e := range entities {
		if e.Sync {
			tables = append(tables, e.Table)
		}
	}
	sort.Strings(tables)

	var b strings.Builder
	if err := dispatchTmpl.Execute(&b, tables); err != nil {
		return codegen.GenFile{}, err
	}
	formatted, err := format.Source([]byte(b.String()))
	if err != nil {
		return codegen.GenFile{}, fmt.Errorf("dispatch.go: %w", err)
	}
	return codegen.GenFile{Name: "dispatch.go", Content: string(formatted)}, nil
}

var dispatchTmpl = mustParse("dispatch.go.tmpl")
