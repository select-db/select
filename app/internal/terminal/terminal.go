package terminal

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	pty "github.com/aymanbagabas/go-pty"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"selectDb/internal/graph"
)

type ShellOption struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

type session struct {
	id   string
	pty  pty.Pty
	cmd  *pty.Cmd
	done chan struct{}
}

type Terminal struct {
	ctx      context.Context
	mu       sync.Mutex
	sessions map[string]*session
	Graph    *graph.Graph
}

func New(g *graph.Graph) *Terminal {
	return &Terminal{
		sessions: make(map[string]*session),
		Graph:    g,
	}
}

func (t *Terminal) SetContext(ctx context.Context) {
	t.ctx = ctx
}

func (t *Terminal) workspaceRootPath() (string, error) {
	if t.Graph == nil || t.Graph.WorkspaceGraph == nil {
		return "", fmt.Errorf("workspace not loaded")
	}
	return graph.WorkspaceRootPath(t.Graph.WorkspaceGraph.ID)
}

func (t *Terminal) GetAvailableShells() []ShellOption {
	if runtime.GOOS == "windows" {
		return getWindowsShells()
	}
	return getUnixShells()
}

func getUnixShells() []ShellOption {
	seen := make(map[string]bool)
	var shells []ShellOption

	// Add $SHELL first if set
	if defaultShell := os.Getenv("SHELL"); defaultShell != "" {
		if _, err := os.Stat(defaultShell); err == nil { // #nosec G703 -- $SHELL is the user's own env on their own machine
			name := filepath.Base(defaultShell)
			shells = append(shells, ShellOption{Path: defaultShell, Name: name})
			seen[defaultShell] = true
		}
	}

	// Parse /etc/shells
	f, err := os.Open("/etc/shells")
	if err != nil {
		if len(shells) == 0 {
			return []ShellOption{{Path: "/bin/sh", Name: "sh"}}
		}
		return shells
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if seen[line] {
			continue
		}
		if _, err := os.Stat(line); err != nil {
			continue
		}
		name := filepath.Base(line)
		shells = append(shells, ShellOption{Path: line, Name: name})
		seen[line] = true
	}

	if len(shells) == 0 {
		return []ShellOption{{Path: "/bin/sh", Name: "sh"}}
	}
	return shells
}

func getWindowsShells() []ShellOption {
	candidates := []struct {
		path string
		name string
	}{
		{"cmd.exe", "Command Prompt"},
		{"powershell.exe", "PowerShell"},
		{"pwsh.exe", "PowerShell Core"},
	}

	var shells []ShellOption
	for _, c := range candidates {
		// exec.LookPath equivalent: these are in PATH by default on Windows
		shells = append(shells, ShellOption{Path: c.path, Name: c.name})
	}
	return shells
}

func (t *Terminal) Create(sessionId string, shell string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.sessions[sessionId]; exists {
		return fmt.Errorf("session %s already exists", sessionId)
	}

	root, err := t.workspaceRootPath()
	if err != nil {
		return fmt.Errorf("failed to resolve workspace root: %w", err)
	}

	if shell == "" {
		shell = os.Getenv("SHELL")
		if shell == "" {
			if runtime.GOOS == "windows" {
				shell = "cmd.exe"
			} else {
				shell = "/bin/sh"
			}
		}
	}

	p, err := pty.New()
	if err != nil {
		return fmt.Errorf("failed to create pty: %w", err)
	}

	cmd := p.Command(shell)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	if err := cmd.Start(); err != nil {
		_ = p.Close()
		return fmt.Errorf("failed to start shell: %w", err)
	}

	sess := &session{
		id:   sessionId,
		pty:  p,
		cmd:  cmd,
		done: make(chan struct{}),
	}
	t.sessions[sessionId] = sess

	go t.readLoop(sess)
	go t.waitForExit(sess)

	return nil
}

func (t *Terminal) readLoop(sess *session) {
	defer func() { recover() }() //nolint:errcheck
	buf := make([]byte, 4096)
	for {
		n, err := sess.pty.Read(buf)
		if n > 0 {
			encoded := base64.StdEncoding.EncodeToString(buf[:n])
			wailsRuntime.EventsEmit(t.ctx, "terminal:output:"+sess.id, encoded)
		}
		if err != nil {
			break
		}
	}
}

func (t *Terminal) waitForExit(sess *session) {
	defer func() { recover() }() //nolint:errcheck
	_ = sess.cmd.Wait()
	close(sess.done)
	wailsRuntime.EventsEmit(t.ctx, "terminal:exit:"+sess.id)

	t.mu.Lock()
	delete(t.sessions, sess.id)
	t.mu.Unlock()
}

func (t *Terminal) Write(sessionId string, data string) error {
	t.mu.Lock()
	sess, ok := t.sessions[sessionId]
	t.mu.Unlock()
	if !ok {
		return fmt.Errorf("session %s not found", sessionId)
	}

	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return fmt.Errorf("failed to decode input: %w", err)
	}

	_, err = sess.pty.Write(decoded)
	return err
}

func (t *Terminal) Resize(sessionId string, cols int, rows int) error {
	t.mu.Lock()
	sess, ok := t.sessions[sessionId]
	t.mu.Unlock()
	if !ok {
		return fmt.Errorf("session %s not found", sessionId)
	}

	return sess.pty.Resize(cols, rows)
}

func (t *Terminal) Destroy(sessionId string) error {
	t.mu.Lock()
	sess, ok := t.sessions[sessionId]
	t.mu.Unlock()
	if !ok {
		return nil
	}

	if sess.cmd.Process != nil {
		_ = sess.cmd.Process.Signal(os.Interrupt)
	}
	_ = sess.pty.Close()

	<-sess.done
	return nil
}

func (t *Terminal) DestroyAll() {
	t.mu.Lock()
	ids := make([]string, 0, len(t.sessions))
	for id := range t.sessions {
		ids = append(ids, id)
	}
	t.mu.Unlock()

	for _, id := range ids {
		_ = t.Destroy(id)
	}
}
