package core

import "testing"

// Which names a CTE body can refer to is a property of the server, not of the
// parse tree. PostgreSQL 16 runs "WITH a AS (SELECT c1 FROM b), b AS (...)"
// against the table b, so a plain WITH must not treat a later name as declared;
// under RECURSIVE the same statement reads the CTE.
func TestCTEScope(t *testing.T) {
	names := []string{"a", "b", "c"}

	tests := []struct {
		name      string
		idx       int
		recursive bool
		want      []string
		notWant   []string
	}{
		{name: "first body sees nothing", idx: 0, want: nil, notWant: []string{"a", "b", "c"}},
		{name: "second body sees the first", idx: 1, want: []string{"a"}, notWant: []string{"b", "c"}},
		{name: "third body sees both before it", idx: 2, want: []string{"a", "b"}, notWant: []string{"c"}},
		{name: "recursive first body sees every name, its own included",
			idx: 0, recursive: true, want: []string{"a", "b", "c"}},
		{name: "recursive last body sees every name",
			idx: 2, recursive: true, want: []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scope := CTEScope(names, tt.idx, tt.recursive)
			for _, name := range tt.want {
				if !scope[name] {
					t.Errorf("%q not in scope, so a body referring to it resolves to no schema and is refused", name)
				}
			}
			for _, name := range tt.notWant {
				if scope[name] {
					t.Errorf("%q in scope, so a read of the real table %q goes unchecked", name, name)
				}
			}
		})
	}
}

// No inspector can pass an index past the end today, but the caller is a
// permission check, so the bound is clamped rather than left to panic.
func TestCTEScope_IndexPastEnd(t *testing.T) {
	scope := CTEScope([]string{"a"}, 5, false)
	if !scope["a"] {
		t.Error("a name declared before the body fell out of scope")
	}
}
