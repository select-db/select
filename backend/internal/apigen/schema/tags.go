package schema

import "strings"

// Tags are the table-level @app directives.
type Tags struct {
	Entity    string
	Sync      bool
	API       []APIOp
	Relations []RelationDecl
	Scope     string
}

// ColumnTags are the column-level @app directives.
type ColumnTags struct {
	Hide      bool
	Immutable bool
	Values    []string
	JSONPaths []string
	Ops       []string
	Lookup    string
	Sort      bool // @app.sort: this column is the resource's default list sort
	SortDesc  bool // @app.sort desc: newest/highest first (default is ascending)
}

// Edge is one join step, table.col -> table.col.
type Edge struct{ FromTable, FromCol, ToTable, ToCol string }

// RelationDecl is one @app.relation line (one path); repeats union by name.
type RelationDecl struct {
	Name string
	Path []Edge
}

// ParseTags reads the table-level directives from a COMMENT string.
func ParseTags(comment string) Tags {
	var t Tags
	for _, d := range directives(comment) {
		head, rest := splitHead(d)
		switch {
		case head == "entity":
			t.Entity = firstToken(rest)
		case head == "sync":
			t.Sync = true
		case head == "scope":
			t.Scope = firstToken(rest)
		case strings.HasPrefix(head, "api."):
			t.API = append(t.API, parseAPI(strings.TrimPrefix(head, "api."), rest)...)
		case head == "relation":
			if r, ok := parseRelation(rest); ok {
				t.Relations = append(t.Relations, r)
			}
		}
	}
	return t
}

// ParseColumnTags reads the column-level directives from a COMMENT string.
func ParseColumnTags(comment string) ColumnTags {
	var c ColumnTags
	for _, d := range directives(comment) {
		head, rest := splitHead(d)
		switch head {
		case "hide":
			c.Hide = true
		case "immutable":
			c.Immutable = true
		case "values":
			c.Values = parseList(rest)
		case "json.paths":
			c.JSONPaths = parseList(rest)
		case "ops":
			c.Ops = parseList(rest)
		case "lookup":
			c.Lookup = firstToken(rest)
		case "sort":
			c.Sort = true
			c.SortDesc = strings.EqualFold(firstToken(rest), "desc")
		}
	}
	return c
}

// directives splits a comment at "@app." boundaries; each result is the text
// of one directive (running until the next "@app." or the end).
func directives(comment string) []string {
	parts := strings.Split(comment, "@app.")
	out := make([]string, 0, len(parts))
	for _, p := range parts[1:] { // [0] is any prose before the first tag
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func splitHead(d string) (head, rest string) {
	d = strings.TrimSpace(d)
	if i := strings.IndexAny(d, " \t\n\r"); i >= 0 {
		return d[:i], strings.TrimSpace(d[i+1:])
	}
	return d, ""
}

func firstToken(s string) string {
	if f := strings.Fields(s); len(f) > 0 {
		return f[0]
	}
	return ""
}

// parseAPI reads one @app.api directive. The op part may be a |-separated group
// (create|update|delete) that shares a single requires clause; each op becomes
// its own APIOp.
func parseAPI(opSpec, rest string) []APIOp {
	var requires []string
	if rest = strings.TrimSpace(rest); strings.HasPrefix(rest, "requires") {
		for _, x := range strings.Split(strings.TrimPrefix(rest, "requires"), ",") {
			if v := strings.TrimSpace(x); v != "" {
				requires = append(requires, v)
			}
		}
	}
	var ops []APIOp
	for _, op := range strings.Split(opSpec, "|") {
		if op = strings.TrimSpace(op); op != "" {
			ops = append(ops, APIOp{Op: op, Requires: requires})
		}
	}
	return ops
}

func parseRelation(rest string) (RelationDecl, bool) {
	i := strings.Index(rest, ":")
	if i < 0 {
		return RelationDecl{}, false
	}
	name := strings.TrimSpace(rest[:i])
	var path []Edge
	for _, seg := range strings.Split(rest[i+1:], ",") {
		e, ok := parseEdge(seg)
		if !ok {
			return RelationDecl{}, false
		}
		path = append(path, e)
	}
	if name == "" || len(path) == 0 {
		return RelationDecl{}, false
	}
	return RelationDecl{Name: name, Path: path}, true
}

func parseEdge(seg string) (Edge, bool) {
	parts := strings.Split(seg, "->")
	if len(parts) != 2 {
		return Edge{}, false
	}
	from, ok1 := parseRef(parts[0])
	to, ok2 := parseRef(parts[1])
	if !ok1 || !ok2 {
		return Edge{}, false
	}
	return Edge{FromTable: from[0], FromCol: from[1], ToTable: to[0], ToCol: to[1]}, true
}

// parseRef reads "table.col" (ignoring any schema prefix and quotes).
func parseRef(s string) ([2]string, bool) {
	s = strings.ReplaceAll(strings.TrimSpace(s), `"`, "")
	p := strings.Split(s, ".")
	if len(p) < 2 {
		return [2]string{}, false
	}
	return [2]string{p[len(p)-2], p[len(p)-1]}, true
}

func parseList(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	if i := strings.Index(s, "]"); i >= 0 {
		s = s[:i]
	}
	var out []string
	for _, x := range strings.Split(s, ",") {
		if v := strings.TrimSpace(x); v != "" {
			out = append(out, v)
		}
	}
	return out
}
