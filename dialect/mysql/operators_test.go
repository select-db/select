package mysql

import (
	"testing"

	core "github.com/selectDb/dialect/core"
)

// TestGetOperatorsForType pins the type-to-operator mapping directly, without
// going through completion. The boolean and numeric families overlap textually
// ("tinyint(1)" contains "int"), so the order of the checks is load-bearing.
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
			columnType: "TINYINT(1)",
			wants:      []string{"IS TRUE", "IS FALSE"},
			rejects:    []string{"BETWEEN"},
		},
		{
			columnType: "boolean",
			wants:      []string{"IS TRUE", "IS FALSE"},
			rejects:    []string{"BETWEEN"},
		},
		{
			columnType: "tinyint(4)",
			wants:      []string{"<", ">", "BETWEEN"},
			rejects:    []string{"IS TRUE"},
		},
		{
			columnType: "int",
			wants:      []string{"<", ">", "BETWEEN", "<=>"},
			rejects:    []string{"IS TRUE", "LIKE"},
		},
		{
			columnType: "bigint unsigned",
			wants:      []string{"<", "BETWEEN"},
			rejects:    []string{"IS TRUE"},
		},
		{
			columnType: "text",
			wants:      []string{"LIKE", "REGEXP", "SOUNDS LIKE"},
			rejects:    []string{"IS TRUE", "BETWEEN"},
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

	var _ core.SQLDialect = d
}
