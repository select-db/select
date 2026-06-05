package tokenanalyzer

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/selectDb/dialect/core"
)

// Analyzer manages a single long-running analyzer subprocess.
// It is safe for concurrent use.
type Analyzer struct {
	mu       sync.Mutex
	execPath string
	args     []string // extra arguments (e.g. script path when execPath is an interpreter)

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
}

// NewAnalyzer creates an Analyzer that will launch execPath with the given args.
func NewAnalyzer(execPath string, args ...string) *Analyzer {
	return &Analyzer{execPath: execPath, args: args}
}


// Close shuts down the subprocess cleanly.
func (c *Analyzer) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.kill()
}

func (c *Analyzer) kill() {
	if c.stdin != nil {
		_ = c.stdin.Close()
		c.stdin = nil
	}
	if c.cmd != nil {
		_ = c.cmd.Process.Kill()
		c.cmd = nil
	}
	c.stdout = nil
}


// ensure starts the subprocess if it is not already running.
func (c *Analyzer) ensure() error {
	if c.cmd != nil && c.cmd.ProcessState == nil {
		return nil
	}
	c.kill()

	cmd := exec.Command(c.execPath, c.args...) //nolint:gosec
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("analyzer stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("analyzer stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("analyzer start: %w", err)
	}
	c.cmd = cmd
	c.stdin = stdin
	c.stdout = bufio.NewScanner(stdout)
	return nil
}

type diagnostic struct {
	RuleID    string `json:"rule_id"`
	Severity  string `json:"severity"` // "error" | "warning" | "hint"
	Message   string `json:"message"`
	StartLine int    `json:"start_line"`
	StartCol  int    `json:"start_col"`
	EndLine   int    `json:"end_line"`
	EndCol    int    `json:"end_col"`
}

type response struct {
	Diagnostics []diagnostic `json:"diagnostics"`
}

// Call sends a raw JSON request to the subprocess and returns the raw JSON response.
// Returns an error on communication failure (the subprocess is killed and will restart
// on the next call).
func (c *Analyzer) Call(req map[string]any) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.callLocked(req)
}

func (c *Analyzer) callLocked(req map[string]any) (json.RawMessage, error) {
	if err := c.ensure(); err != nil {
		return nil, err
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("analyzer marshal: %w", err)
	}

	if _, err := fmt.Fprintf(c.stdin, "%s\n", data); err != nil {
		c.kill()
		return nil, fmt.Errorf("analyzer write: %w", err)
	}

	if !c.stdout.Scan() {
		c.kill()
		return nil, fmt.Errorf("analyzer read: no response")
	}

	raw := make([]byte, len(c.stdout.Bytes()))
	copy(raw, c.stdout.Bytes())
	return json.RawMessage(raw), nil
}

// CallWithTimeout sends a request to the subprocess with a context deadline.
// If the context expires before a response arrives, the subprocess is killed
// (it will restart on the next call) and the context error is returned.
func (c *Analyzer) CallWithTimeout(ctx context.Context, req map[string]any) (json.RawMessage, error) {
	type result struct {
		data json.RawMessage
		err  error
	}

	ch := make(chan result, 1)
	go func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		data, err := c.callLocked(req)
		ch <- result{data, err}
	}()

	select {
	case <-ctx.Done():
		// Deadline expired. We can't interrupt the goroutine holding the mutex,
		// but we can kill the subprocess so the blocked read returns immediately.
		go func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.kill()
		}()
		return nil, ctx.Err()
	case r := <-ch:
		return r.data, r.err
	}
}

// Analyze sends sql + schema context to the subprocess and returns diagnostics.
// Returns nil on any error (graceful degradation).
func (c *Analyzer) Analyze(sql string, d core.SQLDialect, meta core.Metadata) []Diagnostic {
	req := map[string]any{
		"action":         "lint",
		"sql":            sql,
		"dialect":        d.Name(),
		"schema":         MetaToSchemaDict(meta),
		"enums":          core.MetaToEnumDict(meta),
		"default_schema": EffectiveDefaultSchema(meta),
		"functions":      MetaToFunctionNames(meta),
	}

	raw, err := c.Call(req)
	if err != nil {
		return nil
	}

	var resp response
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil
	}

	return responseToDiagnostics(resp)
}

// EffectiveDefaultSchema delegates to core.GetDefaultSchema.
// Kept for backward compatibility with existing callers.
func EffectiveDefaultSchema(meta core.Metadata) string { return core.GetDefaultSchema(meta) }

// MetaToSchemaDict delegates to core.MetaToSchemaDict.
func MetaToSchemaDict(meta core.Metadata) map[string]map[string]map[string]string {
	return core.MetaToSchemaDict(meta)
}

// MetaToFunctionNames delegates to core.MetaToFunctionNames.
func MetaToFunctionNames(meta core.Metadata) []string { return core.MetaToFunctionNames(meta) }

func responseToDiagnostics(resp response) []Diagnostic {
	diags := make([]Diagnostic, 0, len(resp.Diagnostics))
	for _, d := range resp.Diagnostics {
		diags = append(diags, Diagnostic{
			RuleID:    d.RuleID,
			Severity:  parseSeverity(d.Severity),
			Message:   d.Message,
			StartLine: d.StartLine,
			StartCol:  d.StartCol,
			EndLine:   d.EndLine,
			EndCol:    d.EndCol,
		})
	}
	return diags
}

func parseSeverity(s string) Severity {
	switch s {
	case "error":
		return SeverityError
	case "hint":
		return SeverityHint
	default:
		return SeverityWarning
	}
}
