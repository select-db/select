package syncer

import (
	"fmt"
	"go/format"
	"sort"
	"strings"

	"backend/internal/apigen/codegen"
	"backend/internal/apigen/schema"
)

// scopeTarget is one workspace-scoped FK-target table that needs an InWorkspace
// guard. Sing is the sqlc singular (Role), Param the helper's id parameter name
// (roleID).
type scopeTarget struct{ Sing, Param string }

// EmitScope renders internal/syncer/scope/scope.go: one <Target>InWorkspace guard
// per distinct workspace-scoped foreign-key target referenced by a SYNCED entity.
// The decision is pure introspection — a target is guarded iff its table carries
// the tenant column (scoped) — so a new workspace-scoped FK target gets its guard
// generated automatically, with no hand-maintained scope package.
//
// Only synced entities are considered, because the FK-scope guards live in the
// syncer Apply, and Apply exists only for synced tables. A read-only entity
// (e.g. the audit log) never writes its FKs through the API, so its FK targets
// need no guard — and may not even be synced (so have no by-id query). The guard
// leans on the target's workspace-scoped sqlc by-id query (Get<Sing>ByID), so a
// row in another workspace reads as ErrNoRows; `go build` against db/generated
// validates the query exists.
func EmitScope(entities []schema.Entity, scoped map[string]bool) (codegen.GenFile, error) {
	seen := map[string]bool{}
	var targets []scopeTarget
	for _, e := range entities {
		if !e.Sync {
			continue // guards live in Apply, which only synced entities have
		}
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

var scopeTmpl = mustParse("scope.go.tmpl")
