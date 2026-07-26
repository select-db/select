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

var dispatchTmpl = template.Must(template.New("dispatch").Parse(genHeader +
	`package gen

import (
	"context"
	"time"

	"backend/internal/syncer/types"
{{- range .}}
	"backend/internal/syncer/gen/{{.}}"
{{- end}}
)

// Handler is one @app.sync table's write entry points. The hand-written syncer
// builds its dispatch map from Handlers plus the specials (workspace).
type Handler struct {
	Apply        func(context.Context, string, types.Commit, time.Time) (bool, *types.RestoredItem, error)
	ApplyDelete  func(context.Context, string, types.Commit) (bool, *types.RestoredItem, error)
	FetchCurrent func(context.Context, types.Commit) (*types.RestoredItem, error)
}

// Handlers is the dispatch registry for every @app.sync table.
var Handlers = map[string]Handler{
{{- range .}}
	{{printf "%q" .}}: { {{.}}.Apply, {{.}}.ApplyDelete, {{.}}.FetchCurrent},
{{- end}}
}
`))
