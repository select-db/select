package mysql

import (
	"testing"

	core "github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/core/testutil"
	"github.com/selectDb/dialect/mysql/cases"
)

func TestInspect(t *testing.T) {
	dialect := NewDialect()
	meta := core.GetInspectTestMetadata()
	inspector := NewInspector(dialect, meta)

	testCases := core.GetInspectTestCases(meta.DefaultSchema)

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			results := inspector.Inspect(tc.SQL)

			if len(results) != len(tc.Expected) {
				t.Errorf("Expected %d results, got %d", len(tc.Expected), len(results))
				return
			}

			for i, expected := range tc.Expected {
				testutil.CompareResult(t, i, expected, results[i])
			}
		})
	}
}

// TestInspectMySQLSpecific covers syntax that doesn't fit the shared test set
// because it's MySQL-only (ON DUPLICATE KEY UPDATE, multi-table DELETE, etc.).
func TestInspectMySQLSpecific(t *testing.T) {
	dialect := NewDialect()
	meta := core.GetInspectTestMetadata()
	inspector := NewInspector(dialect, meta)
	defaultSchema := meta.DefaultSchema

	testCases := cases.InspectCases(defaultSchema)

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			results := inspector.Inspect(tc.SQL)
			if len(results) != len(tc.Expected) {
				t.Errorf("Expected %d results, got %d", len(tc.Expected), len(results))
				return
			}
			for i, expected := range tc.Expected {
				testutil.CompareResult(t, i, expected, results[i])
			}
		})
	}
}
