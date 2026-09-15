package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// The picker opens on whatever this returns, so what it returns is capped: a
// repository with thousands of branches used to hand every one of them to a
// menu that renders them all.
func TestGetBranches_CapsWhatItLists(t *testing.T) {
	_, workspaceRoot, g, cleanup := setupTestGitRepo(t)
	defer cleanup()

	ctx := context.Background()

	if err := os.WriteFile(filepath.Join(workspaceRoot, "query.sql"), []byte("SELECT 1;"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := runGit(ctx, workspaceRoot, "add", "."); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := runGit(ctx, workspaceRoot, "commit", "-m", "first"); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	// prepareGit wants an origin. Nothing is fetched here, so the URL is never
	// dialled.
	if err := runGit(ctx, workspaceRoot, "remote", "add", "origin", "https://example.invalid/repo.git"); err != nil {
		t.Fatalf("git remote add: %v", err)
	}

	const limit = 5
	original := branchLimit
	branchLimit = limit
	t.Cleanup(func() { branchLimit = original })

	for i := range limit + 3 {
		if err := runGit(ctx, workspaceRoot, "branch", fmt.Sprintf("feature-%02d", i)); err != nil {
			t.Fatalf("git branch: %v", err)
		}
	}

	branches, err := g.GetBranches()
	if err != nil {
		t.Fatalf("GetBranches: %v", err)
	}

	if len(branches) > limit+1 {
		t.Errorf("listed %d branches, want at most %d (the cap, plus the one checked out)", len(branches), limit+1)
	}

	current := ""
	for _, b := range branches {
		if b.IsCurrent {
			current = b.Name
		}
	}
	if current == "" {
		t.Error("the branch that is checked out is missing from the list")
	}
}
