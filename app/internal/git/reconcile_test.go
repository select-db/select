package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// setupRemoteWithFile builds a bare remote repo whose default branch contains a
// single file. Each call produces an independent history (no common ancestor
// with any other remote), which is what a real "switch to a different repo"
// looks like.
func setupRemoteWithFile(t *testing.T, fname, content string) (remoteURL string, cleanup func()) {
	t.Helper()

	tmpDir := t.TempDir()
	bareRepo := filepath.Join(tmpDir, "remote.git")
	workingRepo := filepath.Join(tmpDir, "working")

	ctx := context.Background()

	if err := os.MkdirAll(workingRepo, 0o700); err != nil {
		t.Fatalf("mkdir working: %v", err)
	}
	mustGit(t, workingRepo, "init")
	mustGit(t, workingRepo, "config", "--local", "user.name", "Remote")
	mustGit(t, workingRepo, "config", "--local", "user.email", "remote@example.com")
	if err := os.WriteFile(filepath.Join(workingRepo, fname), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", fname, err)
	}
	mustGit(t, workingRepo, "add", fname)
	mustGit(t, workingRepo, "commit", "-m", "seed "+fname)
	mustGit(t, workingRepo, "branch", "-M", "main")
	mustGit(t, workingRepo, "clone", "--bare", workingRepo, bareRepo)

	_ = ctx
	return bareRepo, func() { os.RemoveAll(tmpDir) }
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	if err := runGit(context.Background(), dir, args...); err != nil {
		t.Fatalf("git %v in %s: %v", args, dir, err)
	}
}

func makeLocalCommit(t *testing.T, dir, fname, content string) {
	t.Helper()
	mustGit(t, dir, "init")
	mustGit(t, dir, "config", "--local", "user.name", "Local")
	mustGit(t, dir, "config", "--local", "user.email", "local@example.com")
	if err := os.WriteFile(filepath.Join(dir, fname), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", fname, err)
	}
	mustGit(t, dir, "add", fname)
	mustGit(t, dir, "commit", "-m", "local "+fname)
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func TestReconcile_FreshLink_EmptyLocal(t *testing.T) {
	wsID, root, g, cleanup := setupTestWorkspace(t)
	defer cleanup()

	remote, rc := setupRemoteWithFile(t, "README.md", "from-remote")
	defer rc()

	res, err := g.ReconcileWorkspaceRemote(wsID, &remote)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if res.Action != ReconcileLinked || !res.Changed {
		t.Fatalf("expected linked+changed, got %+v", res)
	}
	if !exists(filepath.Join(root, "README.md")) {
		t.Error("remote README.md should be checked out")
	}
	if m := readLinkMarker(context.Background(), root); m != remote {
		t.Errorf("marker = %q, want %q", m, remote)
	}
}

func TestReconcile_Idempotent_Noop(t *testing.T) {
	wsID, root, g, cleanup := setupTestWorkspace(t)
	defer cleanup()

	remote, rc := setupRemoteWithFile(t, "README.md", "from-remote")
	defer rc()

	if _, err := g.ReconcileWorkspaceRemote(wsID, &remote); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	res, err := g.ReconcileWorkspaceRemote(wsID, &remote)
	if err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	if res.Action != ReconcileNoop {
		t.Fatalf("expected noop, got %+v", res)
	}
	if !exists(filepath.Join(root, "README.md")) {
		t.Error("content must be preserved on noop")
	}
}

func TestReconcile_SwitchUnrelatedHistory_BacksUpThenRematerializes(t *testing.T) {
	wsID, root, g, cleanup := setupTestWorkspace(t)
	defer cleanup()

	remoteA, rcA := setupRemoteWithFile(t, "a.txt", "alpha")
	defer rcA()
	remoteB, rcB := setupRemoteWithFile(t, "b.txt", "bravo")
	defer rcB()

	// Materialize A, then leave an uncommitted scratch file.
	if _, err := g.ReconcileWorkspaceRemote(wsID, &remoteA); err != nil {
		t.Fatalf("link A: %v", err)
	}
	if !exists(filepath.Join(root, "a.txt")) {
		t.Fatal("a.txt should exist after linking A")
	}
	if err := os.WriteFile(filepath.Join(root, "scratch.txt"), []byte("unpushed"), 0o644); err != nil {
		t.Fatalf("write scratch: %v", err)
	}

	res, err := g.ReconcileWorkspaceRemote(wsID, &remoteB)
	if err != nil {
		t.Fatalf("switch to B: %v", err)
	}
	if res.Action != ReconcileSwitched || !res.Changed {
		t.Fatalf("expected switched+changed, got %+v", res)
	}
	if !exists(filepath.Join(root, "b.txt")) {
		t.Error("b.txt (repo B) should be present after switch")
	}
	if exists(filepath.Join(root, "a.txt")) {
		t.Error("a.txt (repo A) should be gone after switch")
	}
	if res.BackupPath == "" || !exists(res.BackupPath) {
		t.Fatalf("backup path missing: %q", res.BackupPath)
	}
	if !exists(filepath.Join(res.BackupPath, "a.txt")) || !exists(filepath.Join(res.BackupPath, "scratch.txt")) {
		t.Error("backup must contain the previous repo content and unpushed work")
	}
	if m := readLinkMarker(context.Background(), root); m != remoteB {
		t.Errorf("marker = %q, want %q", m, remoteB)
	}
}

func TestReconcile_Unlink_KeepsFilesRemovesOrigin(t *testing.T) {
	wsID, root, g, cleanup := setupTestWorkspace(t)
	defer cleanup()

	remote, rc := setupRemoteWithFile(t, "README.md", "from-remote")
	defer rc()

	if _, err := g.ReconcileWorkspaceRemote(wsID, &remote); err != nil {
		t.Fatalf("link: %v", err)
	}

	res, err := g.ReconcileWorkspaceRemote(wsID, nil)
	if err != nil {
		t.Fatalf("unlink: %v", err)
	}
	if res.Action != ReconcileUnlinked {
		t.Fatalf("expected unlinked, got %+v", res)
	}
	if !exists(filepath.Join(root, "README.md")) {
		t.Error("files must be kept on unlink")
	}
	if o, _ := getGitRemoteOrigin(context.Background(), root); o != "" {
		t.Errorf("origin should be removed, got %q", o)
	}
	if m := readLinkMarker(context.Background(), root); m != "" {
		t.Errorf("marker should be cleared, got %q", m)
	}
}

func TestReconcile_TransientFailure_RetriesAndConverges(t *testing.T) {
	wsID, root, g, cleanup := setupTestWorkspace(t)
	defer cleanup()

	bad := "/nonexistent/repo.git"
	if _, err := g.ReconcileWorkspaceRemote(wsID, &bad); err == nil {
		t.Fatal("expected error for unreachable remote")
	}
	if m := readLinkMarker(context.Background(), root); m != "" {
		t.Errorf("marker must not be written on failure, got %q", m)
	}

	remote, rc := setupRemoteWithFile(t, "README.md", "recovered")
	defer rc()

	res, err := g.ReconcileWorkspaceRemote(wsID, &remote)
	if err != nil {
		t.Fatalf("recovery reconcile: %v", err)
	}
	if res.Action != ReconcileLinked {
		t.Fatalf("expected linked after recovery, got %+v", res)
	}
	if !exists(filepath.Join(root, "README.md")) {
		t.Error("content should converge after recovery")
	}
	if m := readLinkMarker(context.Background(), root); m != remote {
		t.Errorf("marker = %q, want %q", m, remote)
	}
}

func TestReconcile_AdoptRelatedHistory_NoRewipe(t *testing.T) {
	wsID, root, g, cleanup := setupTestWorkspace(t)
	defer cleanup()

	remote, rc := setupRemoteWithFile(t, "README.md", "shared")
	defer rc()

	// Simulate a pre-marker, correctly-linked repo: shares history with the
	// remote, but no marker recorded yet (upgrade path).
	ctx := context.Background()
	mustGit(t, root, "init")
	mustGit(t, root, "config", "--local", "user.name", "Local")
	mustGit(t, root, "config", "--local", "user.email", "local@example.com")
	mustGit(t, root, "remote", "add", "origin", remote)
	mustGit(t, root, "fetch", "origin")
	mustGit(t, root, "checkout", "-b", "main", "--track", "origin/main")

	res, err := g.ReconcileWorkspaceRemote(wsID, &remote)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if res.Action != ReconcileLinked || res.Changed {
		t.Fatalf("expected linked, unchanged (adopt), got %+v", res)
	}
	if res.BackupPath != "" {
		t.Error("adopt path must not create a backup")
	}
	if !exists(filepath.Join(root, "README.md")) {
		t.Error("content must be untouched on adopt")
	}
	if m := readLinkMarker(ctx, root); m != remote {
		t.Errorf("marker = %q, want %q", m, remote)
	}
}

func TestReconcile_OriginAlreadyDesiredButForeignHistory_Rematerializes(t *testing.T) {
	wsID, root, g, cleanup := setupTestWorkspace(t)
	defer cleanup()

	remoteB, rc := setupRemoteWithFile(t, "b.txt", "bravo")
	defer rc()

	// Reproduce the reported bug: origin already points at B (the old buggy
	// code set-url'd it) but the working tree/history is a foreign repo and
	// there is no marker.
	makeLocalCommit(t, root, "a.txt", "alpha")
	mustGit(t, root, "remote", "add", "origin", remoteB)

	res, err := g.ReconcileWorkspaceRemote(wsID, &remoteB)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if res.Action != ReconcileSwitched || !res.Changed {
		t.Fatalf("expected switched+changed, got %+v", res)
	}
	if !exists(filepath.Join(root, "b.txt")) {
		t.Error("repo B content must be materialized")
	}
	if exists(filepath.Join(root, "a.txt")) {
		t.Error("foreign content must be gone")
	}
	if res.BackupPath == "" || !exists(filepath.Join(res.BackupPath, "a.txt")) {
		t.Error("foreign work must be backed up")
	}
}
