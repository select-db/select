package postgresql

import (
	"strings"
	"testing"
)

func operatorsFor(t *testing.T, columnType string) map[string]bool {
	t.Helper()
	d := &Dialect{}
	got := make(map[string]bool)
	for _, op := range d.GetOperatorsForType(columnType) {
		got[op.Text] = true
	}
	return got
}

// TestOperatorsRunOnPostgres pins what a server answers, checked against
// PostgreSQL 16 by running each operator rather than by reading its name. The
// wants and the refusals below are that result; CI needs no server to hold them.
func TestOperatorsRunOnPostgres(t *testing.T) {
	tests := []struct {
		columnType string
		want       []string
		refuse     []string
		why        string
	}{
		{
			columnType: "point",
			want:       []string{"IS NULL", "IS NOT NULL", "<->"},
			refuse:     []string{"=", "<>", "IN", "<", ">", "<=", ">=", "BETWEEN"},
			why:        "point has no equality and no ordering: point = point is an error",
		},
		{
			columnType: "polygon",
			want:       []string{"IS NULL", "<->"},
			refuse:     []string{"=", "<>", "IN", "<", ">="},
			why:        "polygon has neither, and the seven geometric types share only <->",
		},
		{
			columnType: "json",
			want:       []string{"IS NULL", "->", "->>", "#>", "#>>"},
			refuse:     []string{"=", "<>", "IN", "<", ">=", "BETWEEN", "@>", "<@", "?", "?|", "?&"},
			why:        "json holds text: no equality, no ordering, and containment is jsonb only",
		},
		{
			columnType: "jsonb",
			want:       []string{"=", "<>", "IN", "<", ">=", "BETWEEN", "->", "@>", "<@", "?", "?|", "?&", "#>>"},
			why:        "jsonb has all of it, which is what makes the json list wrong for json",
		},
		{
			columnType: "xml",
			want:       []string{"IS NULL", "IS NOT NULL"},
			refuse:     []string{"=", "<>", "IN", "<", ">", "<=", ">="},
			why:        "xml has no equality operator either",
		},
		{
			columnType: "inet",
			want:       []string{"=", "<", ">=", "BETWEEN", "<<", "<<=", ">>", ">>=", "&&"},
			why:        "network containment is the reason to have an inet column",
		},
		{
			columnType: "int4range",
			want:       []string{"=", "<", "BETWEEN", "@>", "<@", "&&", "-|-"},
			why:        "range operators are defined over anyrange, so the concrete type has them",
		},
		{
			columnType: "uuid",
			want:       []string{"=", "<>", "IN", "IS NULL"},
			refuse:     []string{"<", ">", "BETWEEN", "LIKE"},
			why:        "ordering a uuid runs, but it answers no question worth suggesting",
		},
		{
			columnType: "bytea",
			want:       []string{"=", "<", "LIKE", "||"},
			why:        "bytea compares and concatenates",
		},
		{
			columnType: "tsvector",
			want:       []string{"=", "@@"},
			why:        "@@ is the match operator a tsvector column exists for",
		},
		{
			columnType: "integer[]",
			want:       []string{"=", "@>", "<@", "&&", "||", "ANY", "ALL"},
			refuse:     []string{"BETWEEN"},
			why:        "an array takes the array operators, not the element's",
		},
		{
			columnType: "boolean",
			want:       []string{"=", "IS TRUE", "IS NOT FALSE"},
			refuse:     []string{"<", "BETWEEN", "LIKE"},
		},
		{
			columnType: "interval",
			want:       []string{"=", "<", "BETWEEN"},
			refuse:     []string{"LIKE"},
			why:        "interval orders; it used to reach the numeric branch, which gave the same answer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.columnType, func(t *testing.T) {
			got := operatorsFor(t, tt.columnType)
			for _, op := range tt.want {
				if !got[op] {
					t.Errorf("missing %q. %s", op, tt.why)
				}
			}
			for _, op := range tt.refuse {
				if got[op] {
					t.Errorf("offers %q, which PostgreSQL rejects for this type. %s", op, tt.why)
				}
			}
		})
	}
}

// TestTypeNameIsMatchedWhole pins the trap the old classifier fell into: it
// asked whether a type name contained "int" or "char", so a user type was read
// as whichever builtin its spelling happened to hold.
func TestTypeNameIsMatchedWhole(t *testing.T) {
	// Three user types that differ only in spelling. The old classifier gave
	// charge_code the text operators for the "char" in it, print_status the
	// numeric ones for the "int", and order_state neither.
	userTypes := []string{"order_state", "print_status", "charge_code"}
	first := operatorsFor(t, userTypes[0])
	for _, columnType := range userTypes[1:] {
		got := operatorsFor(t, columnType)
		if len(got) != len(first) {
			t.Errorf("%q offers %d operators, %q offers %d: a user type is being read as a builtin its name contains",
				columnType, len(got), userTypes[0], len(first))
		}
		for op := range first {
			if !got[op] {
				t.Errorf("%q lost %q that %q has", columnType, op, userTypes[0])
			}
		}
	}

	// A user type nothing knows about is still a column someone filters on.
	for _, op := range []string{"=", "<", "BETWEEN", "IS NULL"} {
		if !first[op] {
			t.Errorf("an unlisted user type lost %q; an enum or a domain takes it", op)
		}
	}

	// And a builtin whose name holds another is read as itself.
	if operatorsFor(t, "point")["BETWEEN"] {
		t.Error(`point offers BETWEEN: it is being read as a number for the "int" in its name`)
	}
	if operatorsFor(t, "tsvector")["LIKE"] {
		t.Error("tsvector offers LIKE")
	}
}

// TestBaseTypeName covers the spellings introspection actually produces.
func TestBaseTypeName(t *testing.T) {
	tests := []struct {
		in        string
		wantName  string
		wantArray bool
	}{
		{"integer", "integer", false},
		{"numeric(10,2)", "numeric", false},
		{"character varying(255)", "character varying", false},
		// The parameter sits inside the name, so it is removed, not truncated at.
		{"timestamp(3) without time zone", "timestamp without time zone", false},
		{"bit varying(8)", "bit varying", false},
		{"integer[]", "integer", true},
		{"character varying(255)[]", "character varying", true},
		// A type outside the search path arrives schema qualified.
		{"public.order_state", "order_state", false},
		{"  INTEGER  ", "integer", false},
		{"VARCHAR(255)", "varchar", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			name, isArray := baseTypeName(tt.in)
			if name != tt.wantName || isArray != tt.wantArray {
				t.Errorf("baseTypeName(%q) = (%q, %v), want (%q, %v)",
					tt.in, name, isArray, tt.wantName, tt.wantArray)
			}
		})
	}
}

// Both spellings reach a catalog: format_type reports the SQL name, pg_type
// reports the internal one, and a hand-written catalog carries either.
func TestTypeAliasesAgree(t *testing.T) {
	pairs := [][2]string{
		{"character varying(255)", "varchar(255)"},
		{"timestamp without time zone", "timestamp"},
		{"timestamp with time zone", "timestamptz"},
		{"double precision", "float8"},
		{"integer", "int4"},
		{"boolean", "bool"},
		{"bit varying", "varbit"},
	}
	for _, pair := range pairs {
		t.Run(pair[0], func(t *testing.T) {
			a, b := operatorsFor(t, pair[0]), operatorsFor(t, pair[1])
			if len(a) != len(b) {
				t.Fatalf("%q offers %d operators, %q offers %d", pair[0], len(a), pair[1], len(b))
			}
			for op := range a {
				if !b[op] {
					t.Errorf("%q offers %q, %q does not", pair[0], op, pair[1])
				}
			}
		})
	}
}

// Every operator carries text to insert and something to read, since the
// completion list shows both.
func TestEveryOperatorIsPresentable(t *testing.T) {
	d := &Dialect{}
	for _, columnType := range []string{
		"integer", "text", "boolean", "json", "jsonb", "point", "inet",
		"int4range", "uuid", "integer[]", "xml", "order_state",
	} {
		seen := make(map[string]bool)
		for _, op := range d.GetOperatorsForType(columnType) {
			if strings.TrimSpace(op.Text) == "" || strings.TrimSpace(op.InsertText) == "" {
				t.Errorf("%s: %+v has nothing to show or nothing to insert", columnType, op)
			}
			if seen[op.Text] {
				t.Errorf("%s: %q offered twice", columnType, op.Text)
			}
			seen[op.Text] = true
		}
	}
}
