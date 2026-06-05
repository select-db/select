package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"selectDb/backend/db/generated"
	"selectDb/backend/graph"
	"selectDb/backend/src/fs_provider"
	"selectDb/backend/src/server"
	"selectDb/backend/utils"
)

// setupTempAppDataWithServer sets HOME and APP_ENV to a temp dir and creates a current server.
// Returns app root and restore function.
func setupTempAppDataWithServer(t *testing.T) (string, func()) {
	t.Helper()
	oldHome := os.Getenv("HOME")
	oldEnv := os.Getenv("APP_ENV")
	tempHome := t.TempDir()
	os.Setenv("HOME", tempHome)
	os.Setenv("APP_ENV", "test-git")
	appRoot, err := utils.GetAppDataDir()
	if err != nil {
		t.Fatalf("GetAppDataDir: %v", err)
	}
	const testDomain = "test.local"
	serverDir := filepath.Join(appRoot, server.DomainToFolderName(testDomain))
	if err := os.MkdirAll(serverDir, 0o700); err != nil {
		t.Fatalf("create server dir: %v", err)
	}
	if err := server.WriteCurrentDomain(testDomain); err != nil {
		t.Fatalf("write current server: %v", err)
	}
	restore := func() {
		_ = os.Setenv("HOME", oldHome)
		_ = os.Setenv("APP_ENV", oldEnv)
	}
	return appRoot, restore
}

// setupTestGitRepo creates a temporary git repository and returns:
// - workspaceID: the test workspace ID
// - workspaceRoot: the absolute path to the workspace root
// - git: a Git instance configured for testing
// - cleanup: a function to clean up the temp directory
func setupTestGitRepo(t *testing.T) (workspaceID string, workspaceRoot string, git *Git, cleanup func()) {
	t.Helper()

	_, restoreEnv := setupTempAppDataWithServer(t)

	// Use the actual workspace path resolution mechanism
	workspaceID = "test-ws-1"
	var err error
	workspaceRoot, err = graph.WorkspaceRootPath(workspaceID)
	if err != nil {
		restoreEnv()
		t.Fatalf("failed to get workspace root path: %v", err)
	}

	// Create the workspace directory
	if err := os.MkdirAll(workspaceRoot, 0o700); err != nil {
		t.Fatalf("failed to create workspace root: %v", err)
	}

	// Initialize git repository
	ctx := context.Background()
	if err := runGit(ctx, workspaceRoot, "init"); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user (required for commits) - use --local to only affect this repo
	if err := runGit(ctx, workspaceRoot, "config", "--local", "user.name", "Test User"); err != nil {
		t.Fatalf("failed to set git user.name: %v", err)
	}
	if err := runGit(ctx, workspaceRoot, "config", "--local", "user.email", "test@example.com"); err != nil {
		t.Fatalf("failed to set git user.email: %v", err)
	}

	// Create Git instance with the workspace graph so prepareGitLocal can resolve the root.
	git = &Git{
		ctx:        ctx,
		Queries:    &generated.Queries{},
		FSProvider: fs_provider.New(),
		Graph:      &graph.Graph{WorkspaceGraph: &graph.WorkspaceNode{ID: workspaceID}},
	}

	cleanup = func() {
		os.RemoveAll(workspaceRoot)
		restoreEnv()
	}

	return workspaceID, workspaceRoot, git, cleanup
}

func TestGetGitFileStatus_NotGitRepo(t *testing.T) {
	_, restore := setupTempAppDataWithServer(t)
	defer restore()

	workspaceID := "test-ws-not-git"
	workspaceRoot, err := graph.WorkspaceRootPath(workspaceID)
	if err != nil {
		t.Fatalf("failed to get workspace root path: %v", err)
	}

	// Create the directory but don't initialize git
	if err := os.MkdirAll(workspaceRoot, 0o700); err != nil {
		t.Fatalf("failed to create workspace root: %v", err)
	}
	defer os.RemoveAll(workspaceRoot)

	github := &Git{
		ctx:        context.Background(),
		Queries:    &generated.Queries{},
		FSProvider: fs_provider.New(),
		Graph:      &graph.Graph{WorkspaceGraph: &graph.WorkspaceNode{ID: workspaceID}},
	}

	_, err = github.GetGitFileStatus()
	if err == nil {
		t.Fatal("expected error for non-git repository, got nil")
	}
	if err.Error() != "workspace is not a git repository" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGetGitFileStatus_CleanRepo(t *testing.T) {
	_, _, github, cleanup := setupTestGitRepo(t)
	defer cleanup()

	status, err := github.GetGitFileStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.HasChanges {
		t.Error("expected HasChanges=false for clean repo")
	}
	if len(status.Staged) != 0 {
		t.Errorf("expected no staged files, got %d", len(status.Staged))
	}
	if len(status.Unstaged) != 0 {
		t.Errorf("expected no unstaged files, got %d", len(status.Unstaged))
	}
	if len(status.Untracked) != 0 {
		t.Errorf("expected no untracked files, got %d", len(status.Untracked))
	}
}

func TestGetGitFileStatus_StagedAdded(t *testing.T) {
	_, workspaceRoot, github, cleanup := setupTestGitRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create and stage a new file
	testFile := filepath.Join(workspaceRoot, "newfile.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := runGit(ctx, workspaceRoot, "add", "newfile.txt"); err != nil {
		t.Fatalf("failed to stage file: %v", err)
	}

	status, err := github.GetGitFileStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !status.HasChanges {
		t.Error("expected HasChanges=true")
	}
	if len(status.Staged) != 1 {
		t.Fatalf("expected 1 staged file, got %d", len(status.Staged))
	}
	if status.Staged[0].Path != "newfile.txt" {
		t.Errorf("expected path 'newfile.txt', got %q", status.Staged[0].Path)
	}
	if status.Staged[0].Status != "staged" {
		t.Errorf("expected status 'staged', got %q", status.Staged[0].Status)
	}
	if status.Staged[0].PorcelainCode != "A " {
		t.Errorf("expected porcelain code 'A ', got %q", status.Staged[0].PorcelainCode)
	}
	if len(status.Unstaged) != 0 {
		t.Errorf("expected no unstaged files, got %d", len(status.Unstaged))
	}
	if len(status.Untracked) != 0 {
		t.Errorf("expected no untracked files, got %d", len(status.Untracked))
	}
}

func TestGetGitFileStatus_StagedModified(t *testing.T) {
	_, workspaceRoot, github, cleanup := setupTestGitRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create, commit, modify, and stage a file
	testFile := filepath.Join(workspaceRoot, "modified.txt")
	if err := os.WriteFile(testFile, []byte("original"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := runGit(ctx, workspaceRoot, "add", "modified.txt"); err != nil {
		t.Fatalf("failed to stage file: %v", err)
	}
	if err := runGit(ctx, workspaceRoot, "commit", "-m", "initial commit"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Modify and stage again
	if err := os.WriteFile(testFile, []byte("modified"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}
	if err := runGit(ctx, workspaceRoot, "add", "modified.txt"); err != nil {
		t.Fatalf("failed to stage modified file: %v", err)
	}

	status, err := github.GetGitFileStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(status.Staged) != 1 {
		t.Fatalf("expected 1 staged file, got %d", len(status.Staged))
	}
	if status.Staged[0].Path != "modified.txt" {
		t.Errorf("expected path 'modified.txt', got %q", status.Staged[0].Path)
	}
	if status.Staged[0].PorcelainCode != "M " {
		t.Errorf("expected porcelain code 'M ', got %q", status.Staged[0].PorcelainCode)
	}
}

func TestGetGitFileStatus_UnstagedModified(t *testing.T) {
	_, workspaceRoot, github, cleanup := setupTestGitRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create, commit, then modify without staging
	testFile := filepath.Join(workspaceRoot, "unstaged.txt")
	if err := os.WriteFile(testFile, []byte("original"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := runGit(ctx, workspaceRoot, "add", "unstaged.txt"); err != nil {
		t.Fatalf("failed to stage file: %v", err)
	}
	if err := runGit(ctx, workspaceRoot, "commit", "-m", "initial commit"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Modify without staging
	if err := os.WriteFile(testFile, []byte("modified"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	status, err := github.GetGitFileStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(status.Unstaged) != 1 {
		t.Errorf("expected 1 unstaged file, got %d", len(status.Unstaged))
		t.Logf("Staged files: %+v", status.Staged)
		t.Logf("Unstaged files: %+v", status.Unstaged)
		t.FailNow()
	}
	if status.Unstaged[0].Path != "unstaged.txt" {
		t.Errorf("expected path 'unstaged.txt', got %q", status.Unstaged[0].Path)
	}
	if status.Unstaged[0].Status != "unstaged" {
		t.Errorf("expected status 'unstaged', got %q", status.Unstaged[0].Status)
	}
	if status.Unstaged[0].PorcelainCode != " M" {
		t.Errorf("expected porcelain code ' M', got %q", status.Unstaged[0].PorcelainCode)
	}
	if len(status.Staged) != 0 {
		t.Errorf("expected no staged files, got %d", len(status.Staged))
	}
}

func TestGetGitFileStatus_Untracked(t *testing.T) {
	_, workspaceRoot, github, cleanup := setupTestGitRepo(t)
	defer cleanup()

	// Create an untracked file
	testFile := filepath.Join(workspaceRoot, "untracked.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	status, err := github.GetGitFileStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(status.Untracked) != 1 {
		t.Fatalf("expected 1 untracked file, got %d", len(status.Untracked))
	}
	if status.Untracked[0].Path != "untracked.txt" {
		t.Errorf("expected path 'untracked.txt', got %q", status.Untracked[0].Path)
	}
	if status.Untracked[0].Status != "untracked" {
		t.Errorf("expected status 'untracked', got %q", status.Untracked[0].Status)
	}
	if status.Untracked[0].PorcelainCode != "??" {
		t.Errorf("expected porcelain code '??', got %q", status.Untracked[0].PorcelainCode)
	}
	if !status.HasChanges {
		t.Error("expected HasChanges=true")
	}
}

func TestGetGitFileStatus_ModifiedInBoth(t *testing.T) {
	_, workspaceRoot, github, cleanup := setupTestGitRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create, commit, modify, stage, then modify again
	testFile := filepath.Join(workspaceRoot, "both.txt")
	if err := os.WriteFile(testFile, []byte("original"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := runGit(ctx, workspaceRoot, "add", "both.txt"); err != nil {
		t.Fatalf("failed to stage file: %v", err)
	}
	if err := runGit(ctx, workspaceRoot, "commit", "-m", "initial commit"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Modify and stage
	if err := os.WriteFile(testFile, []byte("staged"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}
	if err := runGit(ctx, workspaceRoot, "add", "both.txt"); err != nil {
		t.Fatalf("failed to stage file: %v", err)
	}

	// Modify again without staging
	if err := os.WriteFile(testFile, []byte("unstaged"), 0644); err != nil {
		t.Fatalf("failed to modify file again: %v", err)
	}

	status, err := github.GetGitFileStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(status.Staged) != 1 {
		t.Fatalf("expected 1 staged file, got %d", len(status.Staged))
	}
	if len(status.Unstaged) != 1 {
		t.Fatalf("expected 1 unstaged file, got %d", len(status.Unstaged))
	}
	if status.Staged[0].Path != "both.txt" {
		t.Errorf("staged file path mismatch: got %q", status.Staged[0].Path)
	}
	if status.Unstaged[0].Path != "both.txt" {
		t.Errorf("unstaged file path mismatch: got %q", status.Unstaged[0].Path)
	}
	if status.Staged[0].PorcelainCode != "MM" {
		t.Errorf("expected porcelain code 'MM', got %q", status.Staged[0].PorcelainCode)
	}
}

func TestGetGitFileStatus_MixedStates(t *testing.T) {
	_, workspaceRoot, github, cleanup := setupTestGitRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple files in different states
	// 1. Staged new file
	stagedFile := filepath.Join(workspaceRoot, "staged.txt")
	if err := os.WriteFile(stagedFile, []byte("staged"), 0644); err != nil {
		t.Fatalf("failed to write staged file: %v", err)
	}
	if err := runGit(ctx, workspaceRoot, "add", "staged.txt"); err != nil {
		t.Fatalf("failed to stage file: %v", err)
	}

	// 2. Unstaged modified file
	unstagedFile := filepath.Join(workspaceRoot, "unstaged.txt")
	os.WriteFile(unstagedFile, []byte("original"), 0644)
	runGit(ctx, workspaceRoot, "add", "unstaged.txt")
	// Commit only unstaged.txt, not staged.txt (which should remain staged)
	runGit(ctx, workspaceRoot, "commit", "-m", "commit", "unstaged.txt")
	os.WriteFile(unstagedFile, []byte("modified"), 0644)

	// 3. Untracked file
	untrackedFile := filepath.Join(workspaceRoot, "untracked.txt")
	os.WriteFile(untrackedFile, []byte("untracked"), 0644)

	status, err := github.GetGitFileStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(status.Staged) != 1 {
		t.Errorf("expected 1 staged file, got %d", len(status.Staged))
	}
	if len(status.Unstaged) != 1 {
		t.Errorf("expected 1 unstaged file, got %d", len(status.Unstaged))
	}
	if len(status.Untracked) != 1 {
		t.Errorf("expected 1 untracked file, got %d", len(status.Untracked))
	}
	if !status.HasChanges {
		t.Error("expected HasChanges=true")
	}
}

func TestGetGitFileStatus_PathsWithSpaces(t *testing.T) {
	_, workspaceRoot, github, cleanup := setupTestGitRepo(t)
	defer cleanup()

	// Create a file with spaces in the name
	testFile := filepath.Join(workspaceRoot, "file with spaces.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	status, err := github.GetGitFileStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(status.Untracked) != 1 {
		t.Fatalf("expected 1 untracked file, got %d", len(status.Untracked))
	}
	// Git may quote paths with spaces, but our code should unquote them
	expectedPath := "file with spaces.txt"
	if status.Untracked[0].Path != expectedPath {
		t.Errorf("expected path %q, got %q", expectedPath, status.Untracked[0].Path)
	}
}

func TestGetGitFileStatus_DirectoriesSkipped(t *testing.T) {
	_, workspaceRoot, github, cleanup := setupTestGitRepo(t)
	defer cleanup()

	// Create an untracked directory (git shows it with trailing /)
	dirPath := filepath.Join(workspaceRoot, "untracked-dir")
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	status, err := github.GetGitFileStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Directories should be skipped
	for _, file := range status.Untracked {
		if filepath.Base(file.Path) == "untracked-dir" {
			t.Errorf("directory should be skipped, but found: %q", file.Path)
		}
	}
}

func TestGetGitFileStatus_BranchDetection(t *testing.T) {
	_, workspaceRoot, github, cleanup := setupTestGitRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create initial commit to establish a branch
	testFile := filepath.Join(workspaceRoot, "file.txt")
	os.WriteFile(testFile, []byte("content"), 0644)
	runGit(ctx, workspaceRoot, "add", "file.txt")
	runGit(ctx, workspaceRoot, "commit", "-m", "initial")

	// Create a new branch
	runGit(ctx, workspaceRoot, "checkout", "-b", "test-branch")

	status, err := github.GetGitFileStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.Branch != "test-branch" {
		t.Errorf("expected branch 'test-branch', got %q", status.Branch)
	}
}

func TestGetGitFileStatus_StagedDeleted(t *testing.T) {
	_, workspaceRoot, github, cleanup := setupTestGitRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create, commit, then delete and stage deletion
	testFile := filepath.Join(workspaceRoot, "to-delete.txt")
	os.WriteFile(testFile, []byte("content"), 0644)
	runGit(ctx, workspaceRoot, "add", "to-delete.txt")
	runGit(ctx, workspaceRoot, "commit", "-m", "initial")

	// Delete and stage deletion
	os.Remove(testFile)
	runGit(ctx, workspaceRoot, "add", "to-delete.txt")

	status, err := github.GetGitFileStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(status.Staged) != 1 {
		t.Fatalf("expected 1 staged file, got %d", len(status.Staged))
	}
	if status.Staged[0].Path != "to-delete.txt" {
		t.Errorf("expected path 'to-delete.txt', got %q", status.Staged[0].Path)
	}
	if status.Staged[0].PorcelainCode != "D " {
		t.Errorf("expected porcelain code 'D ', got %q", status.Staged[0].PorcelainCode)
	}
}

func TestGetGitFileStatus_UnstagedDeleted(t *testing.T) {
	_, workspaceRoot, github, cleanup := setupTestGitRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create, commit, then delete without staging
	testFile := filepath.Join(workspaceRoot, "deleted.txt")
	os.WriteFile(testFile, []byte("content"), 0644)
	runGit(ctx, workspaceRoot, "add", "deleted.txt")
	runGit(ctx, workspaceRoot, "commit", "-m", "initial")

	// Delete without staging
	os.Remove(testFile)

	status, err := github.GetGitFileStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(status.Unstaged) != 1 {
		t.Fatalf("expected 1 unstaged file, got %d", len(status.Unstaged))
	}
	if status.Unstaged[0].Path != "deleted.txt" {
		t.Errorf("expected path 'deleted.txt', got %q", status.Unstaged[0].Path)
	}
	if status.Unstaged[0].PorcelainCode != " D" {
		t.Errorf("expected porcelain code ' D', got %q", status.Unstaged[0].PorcelainCode)
	}
}

func TestGetGitFileStatus_Renamed(t *testing.T) {
	_, workspaceRoot, github, cleanup := setupTestGitRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Create, commit, then rename and stage
	oldFile := filepath.Join(workspaceRoot, "old.txt")
	os.WriteFile(oldFile, []byte("content"), 0644)
	runGit(ctx, workspaceRoot, "add", "old.txt")
	runGit(ctx, workspaceRoot, "commit", "-m", "initial")

	// Rename using git mv
	runGit(ctx, workspaceRoot, "mv", "old.txt", "new.txt")

	status, err := github.GetGitFileStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Renamed files should show up in staged with the new path
	if len(status.Staged) != 1 {
		t.Fatalf("expected 1 staged file, got %d", len(status.Staged))
	}
	if status.Staged[0].Path != "new.txt" {
		t.Errorf("expected path 'new.txt', got %q", status.Staged[0].Path)
	}
	// Renamed files have porcelain code "R " or "R " followed by "old -> new"
	if status.Staged[0].PorcelainCode[0] != 'R' {
		t.Errorf("expected porcelain code starting with 'R', got %q", status.Staged[0].PorcelainCode)
	}
}
