package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"testing"
)

// The finder keys placements on case and dialect, so two cases sharing a name
// in one table would be placed as one.
func TestExportCasesAreKeyedUniquely(t *testing.T) {
	var out bytes.Buffer
	if err := exportCases(&out); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	layers := map[string]int{}
	scanner := bufio.NewScanner(&out)
	for scanner.Scan() {
		var known KnownCase
		if err := json.Unmarshal(scanner.Bytes(), &known); err != nil {
			t.Fatalf("%v: %s", err, scanner.Text())
		}
		key := known.Case + "|" + known.Dialect
		if seen[key] {
			t.Errorf("exported twice: %s", key)
		}
		seen[key] = true
		layers[known.Layer]++
	}
	for _, layer := range []string{"permission", "completion", "resolution"} {
		if layers[layer] == 0 {
			t.Errorf("exported no %s case", layer)
		}
	}
}
