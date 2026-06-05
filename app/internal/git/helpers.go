package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// context returns g.ctx if set, otherwise context.Background().
// All methods should call this instead of context.Background() directly
// so that Wails request cancellation propagates to git commands.
func (g *Git) context() context.Context {
	if g.ctx != nil {
		return g.ctx
	}
	return context.Background()
}

// prepareGit validates that git is available, the workspace is a repo, and
// a remote named "origin" is configured. Use for remote operations (push/pull/fetch).
func (g *Git) prepareGit(ctx context.Context) (string, error) {
	workspaceID := g.Graph.WorkspaceGraph.ID

	root, err := g.workspaceRootPath(workspaceID)
	if err != nil {
		return "", err
	}

	if ok, _ := isGitAvailable(ctx); !ok {
		return "", fmt.Errorf("git is not installed or not available in PATH")
	}

	isRepo, _ := isGitRepo(ctx, root)
	if !isRepo {
		return "", fmt.Errorf("workspace is not a git repository")
	}

	remote, _ := getGitRemoteOrigin(ctx, root)
	if remote == "" {
		return "", fmt.Errorf("no git remote named \"origin\" is configured")
	}

	return root, nil
}

// prepareGitLocal validates that git is available and the workspace is a repo,
// without requiring a configured remote. Use for local-only operations
// (staging, committing, reverting).
func (g *Git) prepareGitLocal(ctx context.Context) (string, error) {
	workspaceID := g.Graph.WorkspaceGraph.ID

	root, err := g.workspaceRootPath(workspaceID)
	if err != nil {
		return "", err
	}

	if ok, _ := isGitAvailable(ctx); !ok {
		return "", fmt.Errorf("git is not installed or not available in PATH")
	}

	isRepo, _ := isGitRepo(ctx, root)
	if !isRepo {
		return "", fmt.Errorf("workspace is not a git repository")
	}

	return root, nil
}

// hasAnyCommits reports whether the repository at dir has at least one commit.
func hasAnyCommits(ctx context.Context, dir string) (bool, error) {
	_, err := runGitWithOutput(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return false, nil
	}
	return true, nil
}

// ensureGitRepo initialises a git repository at dir if it is not already one.
func ensureGitRepo(ctx context.Context, dir string) error {
	if ok, _ := isGitRepo(ctx, dir); ok {
		return nil
	}
	if err := runGit(ctx, dir, "init"); err != nil {
		return fmt.Errorf("git init failed: %w", err)
	}
	return nil
}

// remoteDefaultBranch returns the name of the remote's default branch, or ""
// if the remote has no commits or the default branch cannot be determined.
func remoteDefaultBranch(ctx context.Context, dir string) (string, error) {
	out, err := runGitWithOutput(ctx, dir, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err != nil {
		// Auto-detect and set the remote HEAD symref, then retry.
		if err2 := runGit(ctx, dir, "remote", "set-head", "origin", "-a"); err2 != nil {
			return "", nil
		}
		out, err = runGitWithOutput(ctx, dir, "symbolic-ref", "refs/remotes/origin/HEAD")
		if err != nil {
			return "", nil
		}
	}
	branch := strings.TrimPrefix(strings.TrimSpace(out), "refs/remotes/origin/")
	return branch, nil
}

func getCurrentBranch(ctx context.Context, dir string) (string, error) {
	out, err := runGitWithOutput(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to determine current branch: %w", err)
	}
	branch := strings.TrimSpace(out)
	if branch == "" || branch == "HEAD" {
		return "", fmt.Errorf("could not resolve current branch name")
	}
	return branch, nil
}

func isGitRepo(ctx context.Context, dir string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return false, nil
	}

	return strings.TrimSpace(stdout.String()) == "true", nil
}

func getGitRemoteOrigin(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "remote", "get-url", "origin")
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", nil
	}

	return strings.TrimSpace(stdout.String()), nil
}

var (
	gitAvailableOnce sync.Once
	gitAvailable     bool
)

// isGitAvailable checks once whether the git binary is available on the system
// PATH by invoking "git --version". The result is cached for the lifetime of
// the process.
func isGitAvailable(_ context.Context) (bool, error) {
	gitAvailableOnce.Do(func() {
		cmd := exec.CommandContext(context.Background(), "git", "--version")
		gitAvailable = cmd.Run() == nil
	})
	return gitAvailable, nil
}

func runGit(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %v failed: %v (%s)", args, err, stderr.String())
	}
	return nil
}

func runGitWithOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %v failed: %v (%s)", args, err, stderr.String())
	}

	return stdout.String(), nil
}
