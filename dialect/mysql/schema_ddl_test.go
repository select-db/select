package mysql

import (
	"strings"
	"testing"

	core "github.com/selectDb/dialect/core"
)

func TestColumnDDLKeepsCurrentTimestampUnquoted(t *testing.T) {
	now := "CURRENT_TIMESTAMP"
	// MySQL 5.7 reports the default with no DEFAULT_GENERATED in extra.
	c := columnRow{ColumnRow: core.ColumnRow{Name: "at", Type: "timestamp", Nullable: true, Default: &now}, DataType: "timestamp"}
	if got := columnDDL(c, "", false); !strings.Contains(got, "DEFAULT CURRENT_TIMESTAMP") {
		t.Errorf("columnDDL = %q", got)
	}
}

func TestViewDDLIsEmptyWithoutADefinition(t *testing.T) {
	if got := buildViewDDL(viewRow{Schema: "s", Name: "v", Definer: "u@%"}); got != "" {
		t.Errorf("buildViewDDL = %q, want empty when SHOW VIEW is not granted", got)
	}
}

func TestTableDDLLeavesOutExpressionKeys(t *testing.T) {
	ddl := buildTableDDL(
		tableRow{RelationRow: core.RelationRow{Schema: "s", Name: "t"}},
		[]columnRow{{ColumnRow: core.ColumnRow{Name: "a", Type: "int"}}},
		[]indexColumnRow{{IndexColumnRow: core.IndexColumnRow{Index: "k", Position: 1}}},
		nil, false)
	if strings.Contains(ddl, "()") {
		t.Errorf("an expression key wrote invalid DDL:\n%s", ddl)
	}
}
