package syncer

import (
	"backend/internal/apigen/codegen"
	"backend/internal/apigen/schema"
	"fmt"
	"go/format"
	"sort"
	"strings"
)

// authzEntry is one table's required-action AND-list for the registry.
type authzEntry struct {
	Table   string
	Actions []string // core.Action* constant references, in tag order
}

// EmitAuthorize renders authorize.go: the RequiredActions registry mapping
// each @app.sync table to the workspace actions a caller must hold to write it
// (create/update/delete via sync), derived from the `requires` clause on the
// table's @app.api write ops. authorizeCommit composes it with the shared
// preamble, the owner bypass, and the hand-written workspace special.
func EmitAuthorize(entities []schema.Entity) (codegen.GenFile, error) {
	var entries []authzEntry
	for _, e := range entities {
		if !e.Sync {
			continue
		}
		var actions []string
		for _, req := range writeRequires(e) {
			c, ok := actionConst[req]
			if !ok {
				return codegen.GenFile{}, fmt.Errorf("%s: @app.api requires unknown action %q (add it to actionConst)", e.Table, req)
			}
			actions = append(actions, c)
		}
		if len(actions) == 0 {
			continue // no write gating for this table beyond the shared preamble
		}
		entries = append(entries, authzEntry{Table: e.Table, Actions: actions})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Table < entries[j].Table })

	var b strings.Builder
	if err := authorizeTmpl.Execute(&b, entries); err != nil {
		return codegen.GenFile{}, err
	}
	formatted, err := format.Source([]byte(b.String()))
	if err != nil {
		return codegen.GenFile{}, fmt.Errorf("authorize.go: %w", err)
	}
	return codegen.GenFile{Name: "authorize.go", Content: string(formatted)}, nil
}

// writeRequires collects the required actions across a table's write ops
// (create/update/delete), deduped in first-seen order. The read ops (list/get)
// carry no requires and never gate a sync write.
func writeRequires(e schema.Entity) []string {
	seen := map[string]bool{}
	var out []string
	for _, op := range e.API {
		if !schema.IsWriteOp(op.Op) {
			continue
		}
		for _, r := range op.Requires {
			if !seen[r] {
				seen[r] = true
				out = append(out, r)
			}
		}
	}
	return out
}

var authorizeTmpl = mustParse("authorize.go.tmpl")
