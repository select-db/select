package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
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

// dialectTableNames are the <dialect>/cases functions exportDialects reads.
var dialectTableNames = []string{"InspectCases", "SeeCases"}

// A table added to a dialect's cases package that the export does not read
// would be invisible to the finder, which would then probe it as new ground.
func TestExportReadsEveryDialectTable(t *testing.T) {
	for _, d := range exportDialects {
		files, err := filepath.Glob(filepath.Join("..", "..", d.name, "cases", "*.go"))
		if err != nil || len(files) == 0 {
			t.Fatalf("no %s/cases package: %v", d.name, err)
		}
		fset := token.NewFileSet()
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			parsed, err := parser.ParseFile(fset, file, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			for _, decl := range parsed.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if ok && fn.Recv == nil && fn.Name.IsExported() && !slices.Contains(dialectTableNames, fn.Name.Name) {
					t.Errorf("%s/cases.%s is not exported by agentprobe -export-cases", d.name, fn.Name.Name)
				}
			}
		}
	}
}
