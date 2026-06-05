package fs_provider

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type ExecuteCommandParams struct {
	WorkspaceID string   `json:"workspaceId"`
	Command     string   `json:"command"`              // Command to execute (e.g., "grep", "cat", "ls")
	Args        []string `json:"args"`                 // Command arguments
	WorkingDir  string   `json:"workingDir,omitempty"` // Optional: relative path from workspace root
	Timeout     int      `json:"timeout,omitempty"`    // Optional: timeout in seconds (default: 30)
}

type CommandResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
	Error    string `json:"error,omitempty"`
}

// envAllowlist: vars passed to AI-invoked commands. Everything else (tokens,
// cloud creds, app secrets) is dropped so a command can't exfiltrate them.
// Keys upper-cased (Windows env names are case-insensitive).
var envAllowlist = map[string]bool{
	"PATH": true, "HOME": true, "SHELL": true, "USER": true, "LOGNAME": true,
	"LANG": true, "LANGUAGE": true, "TZ": true, "TERM": true, "TMPDIR": true,
	// Windows essentials needed for most programs to start at all
	"SYSTEMROOT": true, "SYSTEMDRIVE": true, "COMSPEC": true, "PATHEXT": true,
	"WINDIR": true, "TEMP": true, "TMP": true, "USERPROFILE": true,
	"HOMEDRIVE": true, "HOMEPATH": true, "APPDATA": true, "LOCALAPPDATA": true,
	"PROGRAMDATA": true, "PROGRAMFILES": true, "PROGRAMFILES(X86)": true,
	"PROGRAMW6432": true, "PUBLIC": true, "ALLUSERSPROFILE": true,
	"NUMBER_OF_PROCESSORS": true, "PROCESSOR_ARCHITECTURE": true,
}

// sanitizedEnv: os.Environ filtered to the allowlist + LC_* locale vars.
func sanitizedEnv() []string {
	var out []string
	for _, kv := range os.Environ() {
		i := strings.IndexByte(kv, '=')
		if i <= 0 {
			continue
		}
		name := strings.ToUpper(kv[:i])
		if envAllowlist[name] || strings.HasPrefix(name, "LC_") {
			out = append(out, kv)
		}
	}
	return out
}

// shell-metachar guards: Command must be a bare program. On Windows it runs
// via "cmd.exe /c", which would otherwise treat these as chaining/redirection.
func hasCommandMeta(s string) bool {
	return s == "" || strings.ContainsAny(s, "&|;<>`$()\"\n\r")
}

func hasWindowsArgMeta(s string) bool {
	return strings.ContainsAny(s, "&|<>^%()\n\r")
}

// ExecuteCommand executes a command in the workspace root directory.
// Commands are executed with a timeout (default 30 seconds).
// The working directory is set to the workspace root, or a subdirectory if workingDir is provided.
func (fsp *FSProvider) ExecuteCommand(params ExecuteCommandParams) (CommandResult, error) {
	// Reject shell metachars in the command (Windows cmd.exe /c chaining).
	if hasCommandMeta(params.Command) {
		return CommandResult{}, fmt.Errorf("invalid command: must be a single program with no shell metacharacters")
	}
	// Get workspace root path
	workspaceRoot, err := fsp.getWorkspaceRoot(params.WorkspaceID)
	if err != nil {
		return CommandResult{}, fmt.Errorf("failed to get workspace root: %w", err)
	}

	// Determine working directory (workspace root or subdirectory)
	workingDir := workspaceRoot
	if params.WorkingDir != "" {
		// Clean the working directory path to prevent path traversal
		cleanPath := filepath.Clean(params.WorkingDir)
		if strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
			return CommandResult{}, fmt.Errorf("invalid working directory: %s", params.WorkingDir)
		}
		workingDir = filepath.Join(workspaceRoot, cleanPath)

		// Verify the directory exists
		if info, err := os.Stat(workingDir); err != nil || !info.IsDir() {
			return CommandResult{}, fmt.Errorf("working directory does not exist: %s", workingDir)
		}
	}

	// Set timeout (default 30 seconds)
	timeout := time.Duration(params.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Build command with arguments
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// cmd.exe /c re-parses the line; reject metachars in args too.
		for _, a := range params.Args {
			if hasWindowsArgMeta(a) {
				return CommandResult{}, fmt.Errorf("invalid argument: shell metacharacters are not allowed")
			}
		}
		allArgs := append([]string{"/c", params.Command}, params.Args...)
		cmd = exec.CommandContext(ctx, "cmd.exe", allArgs...)
	} else {
		// On Unix, execute directly (no shell, args are passed as argv).
		cmd = exec.CommandContext(ctx, params.Command, params.Args...)
	}

	cmd.Dir = workingDir
	cmd.Env = sanitizedEnv()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	exitCode := 0
	if err != nil {
		// Check if context was cancelled (timeout)
		if ctx.Err() == context.DeadlineExceeded {
			return CommandResult{
				Stdout:   stdout.String(),
				Stderr:   stderr.String(),
				ExitCode: -1,
				Error:    fmt.Sprintf("command timed out after %v", timeout),
			}, nil
		}
		if ctx.Err() == context.Canceled {
			return CommandResult{
				Stdout:   stdout.String(),
				Stderr:   stderr.String(),
				ExitCode: -1,
				Error:    "command was cancelled",
			}, nil
		}
		// Check if it's a normal exit error
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			// Command failed to start or was killed
			return CommandResult{
				Stdout:   stdout.String(),
				Stderr:   stderr.String(),
				ExitCode: -1,
				Error:    err.Error(),
			}, nil
		}
	}

	return CommandResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}, nil
}

// getWorkspaceRoot returns the absolute path to the workspace root directory
func (fsp *FSProvider) getWorkspaceRoot(workspaceID string) (string, error) {
	if workspaceID == "" {
		return "", fmt.Errorf("workspace ID is required")
	}

	workspaceURI := fmt.Sprintf("selectdb://workspaces/%s", workspaceID)
	return fsp.GetOSPathFromURI(workspaceURI)
}
