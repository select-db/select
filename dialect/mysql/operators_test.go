package mysql

import "testing"

// TestGetOperatorsForType pins the type-to-operator mapping directly, without
// going through completion. Introspection hands these strings over whole, so
// the width, the enum values and the modifiers are all part of the input.
func TestGetOperatorsForType(t *testing.T) {
	d := NewDialect()

	tests := []struct {
		columnType string
		wants      []string
		rejects    []string
	}{
		{
			columnType: "tinyint(1)",
			wants:      []string{"=", "IS TRUE", "IS FALSE", "IS NOT TRUE", "IS NOT FALSE"},
			rejects:    []string{"<", ">", "BETWEEN", "LIKE"},
		},
		{
			// The same column with a modifier attached, as MySQL reports it.
			columnType: "TINYINT(1) UNSIGNED",
			wants:      []string{"IS TRUE", "IS FALSE"},
			rejects:    []string{"BETWEEN", "LIKE"},
		},
		{
			columnType: "boolean",
			wants:      []string{"IS TRUE", "IS FALSE"},
			rejects:    []string{"BETWEEN"},
		},
		{
			// A wider tinyint is an ordinary integer, not a boolean.
			columnType: "tinyint(4)",
			wants:      []string{"<", ">", "BETWEEN"},
			rejects:    []string{"IS TRUE"},
		},
		{
			columnType: "bigint(20) unsigned",
			wants:      []string{"<", "BETWEEN", "<=>"},
			rejects:    []string{"IS TRUE", "LIKE"},
		},
		{
			columnType: "varchar(255)",
			wants:      []string{"LIKE", "REGEXP", "SOUNDS LIKE"},
			rejects:    []string{"IS TRUE", "BETWEEN"},
		},
		{
			// Enum values are part of the type string. "paint" contains "int"
			// and must not make this numeric.
			columnType: "enum('paint','wall')",
			wants:      []string{"=", "IN", "LIKE", "REGEXP"},
			rejects:    []string{"BETWEEN", "<=>", "IS TRUE"},
		},
		{
			// Likewise "bool" inside a set value must not make this boolean.
			columnType: "set('bool','x')",
			wants:      []string{"=", "IN", "LIKE"},
			rejects:    []string{"IS TRUE", "BETWEEN"},
		},
		{
			// A geometry type contains "int" and is not a number.
			columnType: "point",
			wants:      []string{"=", "IN"},
			rejects:    []string{"BETWEEN", "<=>"},
		},
		{
			columnType: "datetime",
			wants:      []string{"<", ">", "BETWEEN"},
			rejects:    []string{"LIKE", "<=>"},
		},
		{
			columnType: "json",
			wants:      []string{"->", "->>", "JSON_CONTAINS"},
			rejects:    []string{"BETWEEN", "LIKE"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.columnType, func(t *testing.T) {
			got := map[string]bool{}
			for _, op := range d.GetOperatorsForType(tc.columnType) {
				got[op.Text] = true
			}
			for _, want := range tc.wants {
				if !got[want] {
					t.Errorf("missing operator %q for type %q", want, tc.columnType)
				}
			}
			for _, reject := range tc.rejects {
				if got[reject] {
					t.Errorf("unexpected operator %q for type %q", reject, tc.columnType)
				}
			}
		})
	}
}
