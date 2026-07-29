package syncer

import (
	"fmt"
	"go/format"
	"sort"
	"strings"
	"text/template"

	"backend/internal/apigen/codegen"
	"backend/internal/apigen/schema"
)

// scopeTarget is one workspace-scoped FK-target table that needs an InWorkspace
// guard. Sing is the sqlc singular (Role), Param the helper's id parameter name
// (roleID).
type scopeTarget struct{ Sing, Param string }

// EmitScope renders internal/syncer/scope/scope.go: one <Target>InWorkspace guard
// per distinct workspace-scoped foreign-key target across all entities. The
// decision is pure introspection — a target is guarded iff its table carries the
// tenant column (scoped) — so a new workspace-scoped FK target gets its guard
// generated automatically, with no hand-maintained scope package. The guard
// leans on the target's sqlc by-id query (Get<Sing>ByID), which is
// workspace-scoped, so a row in another workspace reads as ErrNoRows; `go build`
// against db/generated validates the query exists (i.e. the target is synced).
func EmitScope(entities []schema.Entity, scoped map[string]bool) (codegen.GenFile, error) {
	seen := map[string]bool{}
	var targets []scopeTarget
	for _, e := range entities {
		for _, f := range e.Fields {
			if f.FK == nil || !scoped[f.FK.Table] || seen[f.FK.Table] {
				continue
			}
			seen[f.FK.Table] = true
			sing := codegen.Pascal(f.FK.Table)
			targets = append(targets, scopeTarget{Sing: sing, Param: lowerFirst(sing) + "ID"})
		}
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].Sing < targets[j].Sing })

	formatted, err := format.Source([]byte(codegen.Render(scopeTmpl, targets)))
	if err != nil {
		return codegen.GenFile{}, fmt.Errorf("scope.go: %w", err)
	}
	return codegen.GenFile{Name: "scope.go", Content: string(formatted)}, nil
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

var scopeTmpl = template.Must(template.New("scope").Parse(genHeader +
	`// Package scope holds the cross-workspace foreign-key guards the generated
// syncer Apply calls: <Target>InWorkspace reports whether a referenced row
// exists in the caller's workspace, so a write can't point an FK at a row in
// another workspace. One guard per workspace-scoped FK target.
package scope

import (
	"context"
	"database/sql"
	"errors"

	"backend/db"
	"backend/db/db_types"
	"backend/db/generated"
)
{{range .}}
// {{.Sing}}InWorkspace reports whether {{.Param}} exists and belongs to
// workspaceID (a workspace-scoped by-id lookup).
func {{.Sing}}InWorkspace(ctx context.Context, {{.Param}}, workspaceID db_types.JSONNullUUID) (bool, error) {
	_, err := db.Queries.Get{{.Sing}}ByID(ctx, generated.Get{{.Sing}}ByIDParams{ID: {{.Param}}, WorkspaceID: workspaceID})
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
{{end}}`))
