// Package query is the runtime the generated REST handlers call to turn a
// request's OData $filter / sort / pagination into a workspace-scoped SQL read.
// It is hand-written and entity-agnostic: the apigen handler emitter generates a
// []Field per resource (from the same IR that produces the OpenAPI
// filterable-fields table), so the API, its docs, and this executor never
// disagree on what is queryable. Values are always bound as parameters, never
// interpolated, so a $filter can't inject SQL.
package query

import "strings"

// Kind is the type class of a filterable column, mirroring apigen's schema.Kind.
// It decides how a literal is parsed and validated.
type Kind int

const (
	KindText Kind = iota
	KindUUID
	KindInt
	KindTime
	KindBool
	KindJSON
	KindInet
)

// Op is an OData $filter operator, in the exact spelling a caller writes (and
// the OpenAPI filterable-fields table advertises).
type Op string

const (
	OpEq         Op = "eq"
	OpNe         Op = "ne"
	OpLt         Op = "lt"
	OpLe         Op = "le"
	OpGt         Op = "gt"
	OpGe         Op = "ge"
	OpIn         Op = "in"
	OpNotIn      Op = "not in"
	OpContains   Op = "contains"
	OpStartsWith Op = "startswith"
	OpEndsWith   Op = "endswith"
)

// Field describes one queryable column a resource exposes. Ops is the set of
// operators allowed on this field, already resolved by the generator from the
// field's kind (and any @app.ops override) — the runtime treats it as the
// authority, so the API can never accept an operator its docs don't advertise.
// Enum, when non-empty, is the closed value set (@app.values); a literal outside
// it is rejected with the allowed values.
type Field struct {
	Name   string
	Column string
	Kind   Kind
	Ops    []Op
	Enum   []string
}

func (f Field) allows(op Op) bool {
	for _, o := range f.Ops {
		if o == op {
			return true
		}
	}
	return false
}

// FieldSet is the lookup a resource's handler builds once from its []Field.
type FieldSet struct {
	byName map[string]Field
	names  []string // declared order, for stable error listings
}

// NewFieldSet indexes fields by their API name. Declaration order is preserved
// for the "known fields are ..." error listing.
func NewFieldSet(fields []Field) *FieldSet {
	fs := &FieldSet{byName: make(map[string]Field, len(fields)), names: make([]string, 0, len(fields))}
	for _, f := range fields {
		fs.byName[f.Name] = f
		fs.names = append(fs.names, f.Name)
	}
	return fs
}

func (fs *FieldSet) lookup(name string) (Field, bool) {
	f, ok := fs.byName[name]
	return f, ok
}

// suggest returns the closest known field name to an unknown one, for a
// "did you mean" hint, or "" when nothing is close enough.
func (fs *FieldSet) suggest(unknown string) string {
	best, bestDist := "", 1<<30
	limit := len(unknown)/2 + 1 // scale tolerance to the typo'd length
	for _, n := range fs.names {
		if d := levenshtein(unknown, n); d < bestDist {
			best, bestDist = n, d
		}
	}
	if bestDist <= limit {
		return best
	}
	return ""
}

func (fs *FieldSet) known() string { return strings.Join(fs.names, ", ") }

// levenshtein is the classic edit distance, case-insensitive (callers compare
// identifiers). Small inputs, so the O(n*m) table is fine.
func levenshtein(a, b string) int {
	a, b = strings.ToLower(a), strings.ToLower(b)
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min3(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}
