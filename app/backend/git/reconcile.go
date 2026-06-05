package git

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// markerKey is the git-config key recording which remote URL the working tree
// has actually been materialized from. It is the client-side source of truth
// for reconciliation, independent of the optimistically-synced
// workspace.git_remote_url DB column. Keying off the marker (what is really on
// disk) instead of the DB column is what makes reconcile self-healing: a
// transient failure simply leaves the marker untouched and retries next sync,
// rather than wedging the client because the DB already claims the new URL.
const markerKey = "select.linkedRemote"

// backupBranchPrefix names the rescue branch created at HEAD before a switch
// re-materializes the workspace from a different repository.
const backupBranchPrefix = "select-backup/"

type ReconcileAction string

const (
	ReconcileNoop     ReconcileAction = "noop"
	ReconcileLinked   ReconcileAction = "linked"
	ReconcileSwitched ReconcileAction = "switched"
	ReconcileUnlinked ReconcileAction = "unlinked"
)

// ReconcileResult describes what reconcile did so the caller can rebuild the
// graph and surface a transparent notification to the user.
type ReconcileResult struct {
	Action     ReconcileAction `json:"action"`
	Changed    bool            `json:"changed"`
	RemoteURL  string          `json:"remoteUrl"`
	BackupPath string          `json:"backupPath"`
	BackupRef  string          `json:"backupRef"`
}

// ReconcileWorkspaceRemote makes the local workspace repository converge to
// desiredURL (or to "no remote" when desiredURL is nil). It is idempotent and
// self-healing, and it never destroys unpushed work: before any switch it
// snapshots the whole workspace (including .git) to a backup directory.
//
// It does not write the workspace.git_remote_url DB column. That column is
// owned by the sync layer (UpsertWorkspaceForSync, @no-track) and by the
// interactive admin actions; writing it here would echo a redundant workspace
// commit back to the server.
func (g *Git) ReconcileWorkspaceRemote(workspaceID string, desiredURL *string) (ReconcileResult, error) {
	ctx := g.context()

	root, err := g.workspaceRootPath(workspaceID)
	if err != nil {
		return ReconcileResult{}, err
	}

	if ok, _ := isGitAvailable(ctx); !ok {
		return ReconcileResult{}, fmt.Errorf("git is not installed or not available in PATH")
	}

	desired := ""
	if desiredURL != nil {
		desired = strings.TrimSpace(*desiredURL)
	}

	isRepo, _ := isGitRepo(ctx, root)
	marker := ""
	if isRepo {
		marker = readLinkMarker(ctx, root)
	}

	if desired == "" {
		return reconcileUnlink(ctx, root, isRepo)
	}

	// Fast path: the working tree is already materialized from desired.
	if isRepo && marker == desired {
		if origin, _ := getGitRemoteOrigin(ctx, root); origin != desired {
			_ = setOrUpdateOrigin(ctx, root, desired)
		}
		return ReconcileResult{Action: ReconcileNoop, RemoteURL: desired}, nil
	}

	if err := ensureGitRepo(ctx, root); err != nil {
		return ReconcileResult{}, err
	}
	if err := setOrUpdateOrigin(ctx, root, desired); err != nil {
		return ReconcileResult{}, err
	}
	if err := runGit(ctx, root, "fetch", "origin"); err != nil {
		return ReconcileResult{}, fmt.Errorf("failed to fetch from remote: %w", err)
	}

	remoteBranch, _ := remoteDefaultBranch(ctx, root)
	hasCommits, _ := hasAnyCommits(ctx, root)

	if isHistoryRelated(ctx, root, remoteBranch, hasCommits) {
		changed := false
		if !hasCommits && remoteBranch != "" {
			if err := checkoutRemoteDefaultBranch(ctx, root); err != nil {
				return ReconcileResult{}, err
			}
			changed = true
		} else {
			if err := setupBranchTracking(ctx, root); err != nil {
				return ReconcileResult{}, err
			}
		}
		if err := writeLinkMarker(ctx, root, desired); err != nil {
			return ReconcileResult{}, err
		}
		return ReconcileResult{Action: ReconcileLinked, Changed: changed, RemoteURL: desired}, nil
	}

	// Unrelated history: the workspace is being pointed at a different repo.
	// Preserve everything first, then re-materialize from desired. If the
	// backup fails we deliberately abort so nothing is lost; reconcile retries.
	backupPath, backupRef, err := backupWorkspace(ctx, root, workspaceID, hasCommits)
	if err != nil {
		return ReconcileResult{}, fmt.Errorf("failed to back up workspace before switch: %w", err)
	}
	if err := rematerialize(ctx, root, desired); err != nil {
		return ReconcileResult{}, err
	}
	if err := writeLinkMarker(ctx, root, desired); err != nil {
		return ReconcileResult{}, err
	}
	return ReconcileResult{
		Action:     ReconcileSwitched,
		Changed:    true,
		RemoteURL:  desired,
		BackupPath: backupPath,
		BackupRef:  backupRef,
	}, nil
}

// reconcileUnlink drops origin and the marker but keeps files and history, so
// the workspace becomes a purely local repository on every member's machine.
func reconcileUnlink(ctx context.Context, root string, isRepo bool) (ReconcileResult, error) {
	if !isRepo {
		return ReconcileResult{Action: ReconcileNoop}, nil
	}
	origin, _ := getGitRemoteOrigin(ctx, root)
	if origin == "" && readLinkMarker(ctx, root) == "" {
		return ReconcileResult{Action: ReconcileNoop}, nil
	}
	if origin != "" {
		_ = runGit(ctx, root, "remote", "remove", "origin")
	}
	clearLinkMarker(ctx, root)
	return ReconcileResult{Action: ReconcileUnlinked}, nil
}

// isHistoryRelated reports whether the local HEAD shares history with the
// desired remote's default branch. An empty local repo (or an empty remote)
// can always adopt; unrelated histories must not be merged or pushed across.
func isHistoryRelated(ctx context.Context, root, remoteBranch string, hasCommits bool) bool {
	if !hasCommits || remoteBranch == "" {
		return true
	}
	_, err := runGitWithOutput(ctx, root, "merge-base", "HEAD", "origin/"+remoteBranch)
	return err == nil
}

// rematerialize wipes the workspace and rebuilds it from desired. Callers must
// have backed up the workspace first.
func rematerialize(ctx context.Context, root, desired string) error {
	if err := clearDirContents(root); err != nil {
		return fmt.Errorf("failed to clear workspace before switch: %w", err)
	}
	if err := runGit(ctx, root, "init"); err != nil {
		return fmt.Errorf("git init failed: %w", err)
	}
	if err := setOrUpdateOrigin(ctx, root, desired); err != nil {
		return err
	}
	if err := runGit(ctx, root, "fetch", "origin"); err != nil {
		return fmt.Errorf("failed to fetch from remote: %w", err)
	}
	branch, _ := remoteDefaultBranch(ctx, root)
	if branch == "" {
		// Desired remote is empty: leave a clean repo tracking origin.
		return nil
	}
	if err := runGit(ctx, root, "checkout", "-b", branch, "--track", "origin/"+branch); err != nil {
		return fmt.Errorf("failed to checkout branch %s: %w", branch, err)
	}
	return nil
}

// backupWorkspace copies the entire workspace (including .git) to a sibling
// backup directory so unpushed commits and uncommitted edits are recoverable.
// When the repo has commits it also tags a rescue branch at HEAD.
func backupWorkspace(ctx context.Context, root, workspaceID string, hasCommits bool) (string, string, error) {
	ts := time.Now().UTC().Format("20060102T150405Z")

	backupRef := ""
	if hasCommits {
		backupRef = backupBranchPrefix + ts
		// best-effort: a missing branch is fine, the full copy below is the
		// real safety net.
		_ = runGit(ctx, root, "branch", backupRef)
	}

	serverRoot := filepath.Dir(filepath.Dir(root))
	dest := filepath.Join(serverRoot, "workspace-backups", fmt.Sprintf("%s-%s", workspaceID, ts))
	if err := copyDir(root, dest); err != nil {
		return "", "", err
	}
	return dest, backupRef, nil
}

func readLinkMarker(ctx context.Context, root string) string {
	out, err := runGitWithOutput(ctx, root, "config", "--local", "--get", markerKey)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func writeLinkMarker(ctx context.Context, root, url string) error {
	if err := runGit(ctx, root, "config", "--local", markerKey, url); err != nil {
		return fmt.Errorf("failed to record linked remote marker: %w", err)
	}
	return nil
}

func clearLinkMarker(ctx context.Context, root string) {
	_ = runGit(ctx, root, "config", "--local", "--unset", markerKey)
}

// clearDirContents removes everything inside dir but keeps dir itself.
func clearDirContents(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			// Copy the link's target contents rather than the link itself;
			// good enough for a recovery snapshot.
			real, err := filepath.EvalSymlinks(path)
			if err != nil {
				return nil
			}
			info, err = os.Stat(real)
			if err != nil || info.IsDir() {
				return nil
			}
			path = real
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if os.IsPermission(err) {
		// The destination already exists read-only (git loose objects are
		// 0444); a non-root user can't reopen it for writing. Drop and recreate.
		if rmErr := os.Remove(dst); rmErr != nil {
			return err
		}
		out, err = os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	}
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
