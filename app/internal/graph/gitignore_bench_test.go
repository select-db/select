package graph

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func benchTree(tb testing.TB) string {
	tb.Helper()

	root := tb.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules\ndist/\n/build\ntarget\n.venv\n__pycache__\ndocs/**/generated\n!vendor/keep\n"), 0o600); err != nil {
		tb.Fatal(err)
	}

	for pkg := 0; pkg < 12; pkg++ {
		for _, sub := range []string{"src", "src/components", "src/lib", "node_modules", "node_modules/dep", "dist", "tests"} {
			dir := filepath.Join(root, fmt.Sprintf("packages/p%d", pkg), filepath.FromSlash(sub))
			if err := os.MkdirAll(dir, 0o755); err != nil {
				tb.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "f.sql"), []byte("select 1"), 0o600); err != nil {
				tb.Fatal(err)
			}
		}
	}
	return root
}

func BenchmarkWalkWithIgnore(b *testing.B) {
	root := benchTree(b)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fsCtx := NewWorkspaceFSFromRoot("ws-bench", root)
		n := 0
		if err := fsCtx.Walk(func(Entry) error { n++; return nil }); err != nil {
			b.Fatal(err)
		}
	}
}
