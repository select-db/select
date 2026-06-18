package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"selectDb/internal/db/generated"
	"selectDb/internal/fs_provider"
	"selectDb/internal/graph"

	"github.com/DATA-DOG/go-sqlmock"
)

// setupTestRemoteRepo creates a "remote" repository that simulates an existing Git repo.
// Returns the path to the remote repo and a cleanup function.
func setupTestRemoteRepo(t *testing.T) (remoteRoot string, cleanup func()) {
	t.Helper()

	// Create a temporary directory for the remote repo
	tmpDir := t.TempDir()
	remoteRoot = filepath.Join(tmpDir, "remote.git")

	ctx := context.Background()

	// Initialize a bare git repository (simulating a remote)
	if err := os.MkdirAll(remoteRoot, 0o700); err != nil {
		t.Fatalf("failed to create remote root: %v", err)
	}

	if err := runGit(ctx, remoteRoot, "init", "--bare"); err != nil {
		t.Fatalf("failed to init bare repo: %v", err)
	}

	cleanup = func() {
		os.RemoveAll(remoteRoot)
	}

	return remoteRoot, cleanup
}

// setupTestRemoteRepoWithContent creates a remote repo with actual commits.
func setupTestRemoteRepoWithContent(t *testing.T) (remoteURL string, cleanup func()) {
	t.Helper()

	tmpDir := t.TempDir()
	bareRepo := filepath.Join(tmpDir, "remote.git")
	workingRepo := filepath.Join(tmpDir, "working")

	ctx := context.Background()

	// Create a working directory to create commits
	if err := os.MkdirAll(workingRepo, 0o700); err != nil {
		t.Fatalf("failed to create working repo: %v", err)
	}

	if err := runGit(ctx, workingRepo, "init"); err != nil {
		t.Fatalf("failed to init working repo: %v", err)
	}

	// Configure git user
	if err := runGit(ctx, workingRepo, "config", "--local", "user.name", "Test User"); err != nil {
		t.Fatalf("failed to set git user.name: %v", err)
	}
	if err := runGit(ctx, workingRepo, "config", "--local", "user.email", "test@example.com"); err != nil {
		t.Fatalf("failed to set git user.email: %v", err)
	}

	// Create initial commit on main branch
	testFile := filepath.Join(workingRepo, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test Repo"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := runGit(ctx, workingRepo, "add", "README.md"); err != nil {
		t.Fatalf("failed to stage file: %v", err)
	}
	if err := runGit(ctx, workingRepo, "commit", "-m", "Initial commit"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}
	// Ensure default branch is "main" so fetch gives origin/main
	if err := runGit(ctx, workingRepo, "branch", "-M", "main"); err != nil {
		t.Fatalf("failed to rename branch to main: %v", err)
	}

	// Create a bare repo from the working repo
	if err := runGit(ctx, workingRepo, "clone", "--bare", workingRepo, bareRepo); err != nil {
		t.Fatalf("failed to clone bare repo: %v", err)
	}

	cleanup = func() {
		os.RemoveAll(tmpDir)
	}

	return bareRepo, cleanup
}

// setupTestWorkspace creates a test workspace without initializing it as a git repo.
// Use for tests that do not call LinkExistingRepo (e.g. HasAnyCommits).
func setupTestWorkspace(t *testing.T) (workspaceID string, workspaceRoot string, git *Git, cleanup func()) {
	t.Helper()

	_, restoreEnv := setupTempAppDataWithServer(t)

	workspaceID = "test-link-ws"
	var err error
	workspaceRoot, err = graph.WorkspaceRootPath(workspaceID)
	if err != nil {
		restoreEnv()
		t.Fatalf("failed to get workspace root path: %v", err)
	}

	// Create the workspace directory
	if err := os.MkdirAll(workspaceRoot, 0o700); err != nil {
		restoreEnv()
		t.Fatalf("failed to create workspace root: %v", err)
	}

	// Create Git instance (Queries not used by HasAnyCommits tests)
	git = &Git{
		ctx:        context.Background(),
		Queries:    &generated.Queries{},
		FSProvider: fs_provider.New(),
		Graph: &graph.Graph{
			WorkspaceGraph: &graph.WorkspaceNode{
				ID: workspaceID,
			},
		},
	}

	cleanup = func() {
		os.RemoveAll(workspaceRoot)
		restoreEnv()
	}

	return workspaceID, workspaceRoot, git, cleanup
}

// setupTestWorkspaceForLink is like setupTestWorkspace but wires Queries to a sqlmock
// and expects one UpdateWorkspaceGitRemote so LinkExistingRepo can run without a real DB.
func setupTestWorkspaceForLink(t *testing.T) (workspaceID string, workspaceRoot string, git *Git, cleanup func()) {
	t.Helper()

	_, restoreEnv := setupTempAppDataWithServer(t)

	workspaceID = "test-link-ws"
	var err error
	workspaceRoot, err = graph.WorkspaceRootPath(workspaceID)
	if err != nil {
		restoreEnv()
		t.Fatalf("failed to get workspace root path: %v", err)
	}

	if err := os.MkdirAll(workspaceRoot, 0o700); err != nil {
		t.Fatalf("failed to create workspace root: %v", err)
	}

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	// LinkExistingRepo calls UpdateWorkspaceGitRemote (UPDATE workspace SET git_remote_url = ? WHERE id = ?)
	mock.ExpectExec("UPDATE workspace").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	git = &Git{
		ctx:        context.Background(),
		Queries:    generated.New(db),
		FSProvider: fs_provider.New(),
		Graph: &graph.Graph{
			WorkspaceGraph: &graph.WorkspaceNode{
				ID: workspaceID,
			},
		},
	}

	cleanup = func() {
		_ = db.Close()
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("sqlmock: %v", err)
		}
		os.RemoveAll(workspaceRoot)
		restoreEnv()
	}

	return workspaceID, workspaceRoot, git, cleanup
}

func TestLinkExistingRepo_UninitializedWorkspace(t *testing.T) {
	// Setup workspace (not a git repo yet)
	_, workspaceRoot, git, cleanupWorkspace := setupTestWorkspaceForLink(t)
	defer cleanupWorkspace()

	// Setup remote with content
	remoteURL, cleanupRemote := setupTestRemoteRepoWithContent(t)
	defer cleanupRemote()

	// Link to existing repo
	status, err := git.LinkExistingRepo(LinkExistingParams{
		RemoteURL: remoteURL,
	})
	if err != nil {
		t.Fatalf("LinkExistingRepo failed: %v", err)
	}
	if status.Scenario == "checkout" {
		if err := git.CompleteLinkExistingRepo("checkout"); err != nil {
			t.Fatalf("CompleteLinkExistingRepo failed: %v", err)
		}
	}

	ctx := context.Background()

	// Verify it's now a git repo
	isRepo, _ := isGitRepo(ctx, workspaceRoot)
	if !isRepo {
		t.Error("workspace should be a git repository")
	}

	// Verify origin is set
	origin, err := getGitRemoteOrigin(ctx, workspaceRoot)
	if err != nil {
		t.Fatalf("failed to get origin: %v", err)
	}
	if origin != remoteURL {
		t.Errorf("expected origin %q, got %q", remoteURL, origin)
	}

	// Verify we checked out the default branch
	branch, err := getCurrentBranch(ctx, workspaceRoot)
	if err != nil {
		t.Fatalf("failed to get current branch: %v", err)
	}
	if branch != "main" && branch != "master" {
		t.Errorf("expected branch 'main' or 'master', got %q", branch)
	}

	// Verify the README file exists (was checked out)
	readmePath := filepath.Join(workspaceRoot, "README.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		t.Error("README.md should exist after checkout")
	}

	// Verify branch tracking is set up
	trackingBranch, err := runGitWithOutput(ctx, workspaceRoot, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		t.Fatalf("failed to get tracking branch: %v", err)
	}
	expectedTracking := "origin/" + branch
	if trackingBranch != expectedTracking+"\n" {
		t.Errorf("expected tracking branch %q, got %q", expectedTracking, trackingBranch)
	}
}

func TestLinkExistingRepo_EmptyInitializedRepo(t *testing.T) {
	// Setup workspace and initialize as empty git repo
	_, workspaceRoot, git, cleanupWorkspace := setupTestWorkspaceForLink(t)
	defer cleanupWorkspace()

	ctx := context.Background()

	// Initialize as git repo but don't commit anything
	if err := runGit(ctx, workspaceRoot, "init"); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Setup remote with content
	remoteURL, cleanupRemote := setupTestRemoteRepoWithContent(t)
	defer cleanupRemote()

	// Link to existing repo
	status, err := git.LinkExistingRepo(LinkExistingParams{
		RemoteURL: remoteURL,
	})
	if err != nil {
		t.Fatalf("LinkExistingRepo failed: %v", err)
	}
	if status.Scenario == "checkout" {
		if err := git.CompleteLinkExistingRepo("checkout"); err != nil {
			t.Fatalf("CompleteLinkExistingRepo failed: %v", err)
		}
	}

	// Verify origin is set
	origin, err := getGitRemoteOrigin(ctx, workspaceRoot)
	if err != nil {
		t.Fatalf("failed to get origin: %v", err)
	}
	if origin != remoteURL {
		t.Errorf("expected origin %q, got %q", remoteURL, origin)
	}

	// Verify we checked out the default branch
	branch, err := getCurrentBranch(ctx, workspaceRoot)
	if err != nil {
		t.Fatalf("failed to get current branch: %v", err)
	}
	if branch != "main" && branch != "master" {
		t.Errorf("expected branch 'main' or 'master', got %q", branch)
	}

	// Verify the README file exists
	readmePath := filepath.Join(workspaceRoot, "README.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		t.Error("README.md should exist after checkout")
	}
}

func TestLinkExistingRepo_RepoWithLocalCommits(t *testing.T) {
	// Setup workspace with existing commits
	_, workspaceRoot, git, cleanupWorkspace := setupTestWorkspaceForLink(t)
	defer cleanupWorkspace()

	ctx := context.Background()

	// Initialize and create a local commit
	if err := runGit(ctx, workspaceRoot, "init"); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user
	if err := runGit(ctx, workspaceRoot, "config", "--local", "user.name", "Local User"); err != nil {
		t.Fatalf("failed to set git user.name: %v", err)
	}
	if err := runGit(ctx, workspaceRoot, "config", "--local", "user.email", "local@example.com"); err != nil {
		t.Fatalf("failed to set git user.email: %v", err)
	}

	// Create local file and commit
	localFile := filepath.Join(workspaceRoot, "local.txt")
	if err := os.WriteFile(localFile, []byte("local content"), 0644); err != nil {
		t.Fatalf("failed to write local file: %v", err)
	}
	if err := runGit(ctx, workspaceRoot, "add", "local.txt"); err != nil {
		t.Fatalf("failed to stage file: %v", err)
	}
	if err := runGit(ctx, workspaceRoot, "commit", "-m", "Local commit"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Get current branch name before linking
	localBranch, err := getCurrentBranch(ctx, workspaceRoot)
	if err != nil {
		t.Fatalf("failed to get current branch: %v", err)
	}

	// Setup remote with content
	remoteURL, cleanupRemote := setupTestRemoteRepoWithContent(t)
	defer cleanupRemote()

	// Link to existing repo
	status, err := git.LinkExistingRepo(LinkExistingParams{
		RemoteURL: remoteURL,
	})
	if err != nil {
		t.Fatalf("LinkExistingRepo failed: %v", err)
	}
	if status.Scenario == "checkout" {
		if err := git.CompleteLinkExistingRepo("checkout"); err != nil {
			t.Fatalf("CompleteLinkExistingRepo failed: %v", err)
		}
	}

	// Verify origin is set
	origin, err := getGitRemoteOrigin(ctx, workspaceRoot)
	if err != nil {
		t.Fatalf("failed to get origin: %v", err)
	}
	if origin != remoteURL {
		t.Errorf("expected origin %q, got %q", remoteURL, origin)
	}

	// Verify we're still on the same branch
	currentBranch, err := getCurrentBranch(ctx, workspaceRoot)
	if err != nil {
		t.Fatalf("failed to get current branch: %v", err)
	}
	if currentBranch != localBranch {
		t.Errorf("expected to stay on branch %q, got %q", localBranch, currentBranch)
	}

	// Verify local commit still exists
	if _, err := os.Stat(localFile); os.IsNotExist(err) {
		t.Error("local.txt should still exist")
	}

	// Verify remote was fetched (remote branches should exist)
	_, err = runGitWithOutput(ctx, workspaceRoot, "rev-parse", "--verify", "origin/main")
	if err != nil {
		_, err = runGitWithOutput(ctx, workspaceRoot, "rev-parse", "--verify", "origin/master")
		if err != nil {
			t.Error("remote branch should exist after fetch")
		}
	}
}

func TestLinkExistingRepo_RepoWithMatchingBranch(t *testing.T) {
	// Setup workspace with a 'main' branch
	_, workspaceRoot, git, cleanupWorkspace := setupTestWorkspaceForLink(t)
	defer cleanupWorkspace()

	ctx := context.Background()

	// Initialize and create a local commit on main branch
	if err := runGit(ctx, workspaceRoot, "init"); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}
	if err := runGit(ctx, workspaceRoot, "config", "--local", "user.name", "Local User"); err != nil {
		t.Fatalf("failed to set git user.name: %v", err)
	}
	if err := runGit(ctx, workspaceRoot, "config", "--local", "user.email", "local@example.com"); err != nil {
		t.Fatalf("failed to set git user.email: %v", err)
	}

	// Rename default branch to 'main' to match remote
	if err := runGit(ctx, workspaceRoot, "checkout", "-b", "main"); err != nil {
		t.Fatalf("failed to create main branch: %v", err)
	}

	// Create local commit
	localFile := filepath.Join(workspaceRoot, "local.txt")
	if err := os.WriteFile(localFile, []byte("local"), 0644); err != nil {
		t.Fatalf("failed to write local file: %v", err)
	}
	if err := runGit(ctx, workspaceRoot, "add", "local.txt"); err != nil {
		t.Fatalf("failed to stage file: %v", err)
	}
	if err := runGit(ctx, workspaceRoot, "commit", "-m", "Local commit"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Setup remote with content (also has main branch)
	remoteURL, cleanupRemote := setupTestRemoteRepoWithContent(t)
	defer cleanupRemote()

	// Link to existing repo
	status, err := git.LinkExistingRepo(LinkExistingParams{
		RemoteURL: remoteURL,
	})
	if err != nil {
		t.Fatalf("LinkExistingRepo failed: %v", err)
	}
	if status.Scenario == "checkout" {
		if err := git.CompleteLinkExistingRepo("checkout"); err != nil {
			t.Fatalf("CompleteLinkExistingRepo failed: %v", err)
		}
	}

	// Verify branch tracking is set up for main
	trackingBranch, err := runGitWithOutput(ctx, workspaceRoot, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		t.Fatalf("failed to get tracking branch: %v", err)
	}
	if trackingBranch != "origin/main\n" {
		t.Errorf("expected tracking branch 'origin/main', got %q", trackingBranch)
	}
}

func TestLinkExistingRepo_AlreadyLinked(t *testing.T) {
	// Setup workspace already linked to a remote
	_, workspaceRoot, git, cleanupWorkspace := setupTestWorkspaceForLink(t)
	defer cleanupWorkspace()

	ctx := context.Background()

	// Initialize and link to first remote
	if err := runGit(ctx, workspaceRoot, "init"); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	firstRemote, cleanupFirst := setupTestRemoteRepo(t)
	defer cleanupFirst()

	if err := runGit(ctx, workspaceRoot, "remote", "add", "origin", firstRemote); err != nil {
		t.Fatalf("failed to add first remote: %v", err)
	}

	// Now link to a different remote
	secondRemote, cleanupSecond := setupTestRemoteRepoWithContent(t)
	defer cleanupSecond()

	status, err := git.LinkExistingRepo(LinkExistingParams{
		RemoteURL: secondRemote,
	})
	if err != nil {
		t.Fatalf("LinkExistingRepo failed: %v", err)
	}
	if status.Scenario == "checkout" {
		if err := git.CompleteLinkExistingRepo("checkout"); err != nil {
			t.Fatalf("CompleteLinkExistingRepo failed: %v", err)
		}
	}

	// Verify origin was updated to the second remote
	origin, err := getGitRemoteOrigin(ctx, workspaceRoot)
	if err != nil {
		t.Fatalf("failed to get origin: %v", err)
	}
	if origin != secondRemote {
		t.Errorf("expected origin %q, got %q", secondRemote, origin)
	}

	// Verify we checked out the default branch from the new remote
	branch, err := getCurrentBranch(ctx, workspaceRoot)
	if err != nil {
		t.Fatalf("failed to get current branch: %v", err)
	}
	if branch != "main" && branch != "master" {
		t.Errorf("expected branch 'main' or 'master', got %q", branch)
	}
}

func TestLinkExistingRepo_GitNotAvailable(t *testing.T) {
	// @todo mock fn
	t.Skip("Requires mocking git availability")
}

func TestLinkExistingRepo_InvalidRemoteURL(t *testing.T) {
	_, _, git, cleanup := setupTestWorkspaceForLink(t)
	defer cleanup()

	// Try to link with an invalid/non-existent remote URL
	_, err := git.LinkExistingRepo(LinkExistingParams{
		RemoteURL: "/nonexistent/repo.git",
	})

	// Should fail during fetch
	if err == nil {
		t.Fatal("expected error for invalid remote URL, got nil")
	}

	// Error should mention fetch failure
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

func TestHasAnyCommits_NoCommits(t *testing.T) {
	_, workspaceRoot, _, cleanup := setupTestWorkspace(t)
	defer cleanup()

	ctx := context.Background()

	// Initialize empty repo
	if err := runGit(ctx, workspaceRoot, "init"); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	hasCommits, err := hasAnyCommits(ctx, workspaceRoot)
	if err != nil {
		t.Fatalf("hasAnyCommits failed: %v", err)
	}

	if hasCommits {
		t.Error("expected hasCommits=false for empty repo")
	}
}

func TestHasAnyCommits_WithCommits(t *testing.T) {
	_, workspaceRoot, _, cleanup := setupTestWorkspace(t)
	defer cleanup()

	ctx := context.Background()

	// Initialize and create a commit
	if err := runGit(ctx, workspaceRoot, "init"); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}
	if err := runGit(ctx, workspaceRoot, "config", "--local", "user.name", "Test"); err != nil {
		t.Fatalf("failed to set git user.name: %v", err)
	}
	if err := runGit(ctx, workspaceRoot, "config", "--local", "user.email", "test@test.com"); err != nil {
		t.Fatalf("failed to set git user.email: %v", err)
	}

	testFile := filepath.Join(workspaceRoot, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := runGit(ctx, workspaceRoot, "add", "test.txt"); err != nil {
		t.Fatalf("failed to stage file: %v", err)
	}
	if err := runGit(ctx, workspaceRoot, "commit", "-m", "test commit"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	hasCommits, err := hasAnyCommits(ctx, workspaceRoot)
	if err != nil {
		t.Fatalf("hasAnyCommits failed: %v", err)
	}

	if !hasCommits {
		t.Error("expected hasCommits=true for repo with commits")
	}
}
