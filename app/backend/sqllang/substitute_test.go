package sqllang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"selectDb/backend/graph"
	"selectDb/backend/server"
	"selectDb/backend/utils"
)

func TestLineColToIndex(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		line   int
		col    int
		expect int
	}{
		{"empty string line 1 col 0", "", 1, 0, 0},
		{"single line col 0", "abc", 1, 0, 0},
		{"single line col 2", "abc", 1, 2, 2},
		{"two lines line 1 col 1", "a\nb", 1, 1, 1},
		{"two lines line 2 col 0", "a\nb", 2, 0, 2},
		{"two lines line 2 col 1", "a\nb", 2, 1, 3},
		{"three lines line 3 col 0", "x\ny\nz", 3, 0, 4},
		{"out of range line", "ab", 5, 0, 0},
		{"col beyond line", "ab", 1, 10, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lineColToIndex(tt.s, tt.line, tt.col)
			if got != tt.expect {
				t.Errorf("lineColToIndex(%q, %d, %d) = %d, want %d", tt.s, tt.line, tt.col, got, tt.expect)
			}
		})
	}
}

func TestIndexToLineCol(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		index    int
		wantLine int
		wantCol  int
	}{
		{"empty index 0", "", 0, 1, 0},
		{"single line index 1", "abc", 1, 1, 1},
		{"two lines index 2", "a\nb", 2, 2, 0},
		{"two lines index 4", "a\nb", 4, 2, 1},
		{"index past end", "ab", 10, 1, 2},
		{"negative index", "ab", -1, 1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line, col := indexToLineCol(tt.s, tt.index)
			if line != tt.wantLine || col != tt.wantCol {
				t.Errorf("indexToLineCol(%q, %d) = (%d, %d), want (%d, %d)", tt.s, tt.index, line, col, tt.wantLine, tt.wantCol)
			}
		})
	}
}

func TestLineColRoundtrip(t *testing.T) {
	s := "WITH x AS (\n  SELECT 1\n)\nSELECT *"
	lines := splitTestLines(s)
	for line := 1; line <= len(lines); line++ {
		maxCol := len(lines[line-1])
		for col := 0; col <= maxCol; col++ {
			idx := lineColToIndex(s, line, col)
			gotLine, gotCol := indexToLineCol(s, idx)
			if gotLine != line || gotCol != col {
				t.Errorf("roundtrip (line=%d, col=%d) -> index %d -> (%d, %d)", line, col, idx, gotLine, gotCol)
			}
		}
	}
}

func splitTestLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return out
}

func TestTrackCaretThroughReplacements(t *testing.T) {
	tests := []struct {
		name      string
		original  string
		resolved  string
		repls     []subRepl
		caretLine int
		caretCol  int
		wantLine  int
		wantCol   int
	}{
		{
			name:      "no replacements",
			original:  "SELECT 1",
			resolved:  "SELECT 1",
			repls:     nil,
			caretLine: 1, caretCol: 3,
			wantLine: 1, wantCol: 3,
		},
		{
			name:      "caret after replacement (expansion)",
			original:  "WITH ($x) AS t SELECT t.",
			resolved:  "WITH (SELECT id, name FROM users) AS t SELECT t.",
			repls:     []subRepl{{Start: 6, End: 8, New: "SELECT id, name FROM users"}},
			caretLine: 1, caretCol: 22,
			wantLine: 1, wantCol: 46,
		},
		{
			name:      "caret before replacement",
			original:  "WITH ($x) AS t",
			resolved:  "WITH (SELECT 1) AS t",
			repls:     []subRepl{{Start: 6, End: 8, New: "SELECT 1"}},
			caretLine: 1, caretCol: 2,
			wantLine: 1, wantCol: 2,
		},
		{
			name:      "caret inside replacement moves to start",
			original:  "WITH ($x) AS t",
			resolved:  "WITH (SELECT 1) AS t",
			repls:     []subRepl{{Start: 6, End: 8, New: "SELECT 1"}},
			caretLine: 1, caretCol: 7,
			wantLine: 1, wantCol: 6,
		},
		{
			name:      "two lines caret on second line after first line expansion",
			original:  "WITH ($cte) AS cu\nSELECT cu.",
			resolved:  "WITH (SELECT id FROM t) AS cu\nSELECT cu.",
			repls:     []subRepl{{Start: 6, End: 11, New: "SELECT id FROM t"}},
			caretLine: 2, caretCol: 9,
			wantLine: 2, wantCol: 8,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLine, gotCol := trackCaretThroughReplacements(tt.original, tt.resolved, tt.repls, tt.caretLine, tt.caretCol)
			if gotLine != tt.wantLine || gotCol != tt.wantCol {
				t.Errorf("trackCaretThroughReplacements(...) = (%d, %d), want (%d, %d)", gotLine, gotCol, tt.wantLine, tt.wantCol)
			}
		})
	}
}

func TestEscapeSQLValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "'hello'"},
		{"John O'Brien", "'John O''Brien'"},
		{"It's a 'test'", "'It''s a ''test'''"},
		{"123", "123"},
		{"123.45", "123.45"},
		{"-456", "-456"},
		{"true", "TRUE"},
		{"false", "FALSE"},
		{"TRUE", "TRUE"},
		{"False", "FALSE"},
		{"  true  ", "TRUE"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeSQLValue(tt.input)
			if result != tt.expected {
				t.Errorf("escapeSQLValue(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

type substCase struct {
	name           string
	envVars        map[string]string
	workspaceFiles map[string]string
	input          string
	wantSQL        string
	wantErr        bool
	wantErrSubstr  string
	caretLine      int
	caretCol       int
	wantCaretLine  int
	wantCaretCol   int
}

func TestSubstituteVariablesSQL_Cases(t *testing.T) {
	cases := []substCase{
		{
			name:      "no refs",
			envVars:   map[string]string{"X": "1"},
			input:     "SELECT 1 FROM t",
			wantSQL:   "SELECT 1 FROM t",
			caretLine: -1,
		},
		{
			name:      "single env var",
			envVars:   map[string]string{"NAME": "alice"},
			input:     "SELECT * FROM u WHERE n = $NAME",
			wantSQL:   "SELECT * FROM u WHERE n = 'alice'",
			caretLine: -1,
		},
		{
			name:      "multiple env vars",
			envVars:   map[string]string{"A": "1", "B": "2"},
			input:     "SELECT $A, $B",
			wantSQL:   "SELECT 1, 2",
			caretLine: -1,
		},
		{
			name:          "variable not found",
			envVars:       map[string]string{},
			input:         "SELECT $MISSING",
			wantErr:       true,
			wantErrSubstr: "not found",
			caretLine:     -1,
		},
		{
			name:      "empty input",
			envVars:   map[string]string{"X": "1"},
			input:     "",
			wantSQL:   "",
			caretLine: -1,
		},
		{
			name:      "SQL escaping",
			envVars:   map[string]string{"NUM": "42", "STR": "O'Brien", "BOOL": "true"},
			input:     "SELECT $NUM, $STR, $BOOL",
			wantSQL:   "SELECT 42, 'O''Brien', TRUE",
			caretLine: -1,
		},
		{
			name:          "caret no refs",
			envVars:       map[string]string{"X": "1"},
			input:         "SELECT 1",
			wantSQL:       "SELECT 1",
			caretLine:     1,
			caretCol:      3,
			wantCaretLine: 1,
			wantCaretCol:  3,
		},
		{
			name:          "caret after ref (expansion)",
			envVars:       map[string]string{"x": "SELECT 1"},
			input:         "WITH ($x) AS t SELECT t.",
			wantSQL:       "WITH ('SELECT 1') AS t SELECT t.",
			caretLine:     1,
			caretCol:      22,
			wantCaretLine: 1,
			wantCaretCol:  30,
		},
		{
			name:          "caret before ref",
			envVars:       map[string]string{"x": "SELECT 1"},
			input:         "WITH ($x) AS t",
			wantSQL:       "WITH ('SELECT 1') AS t",
			caretLine:     1,
			caretCol:      2,
			wantCaretLine: 1,
			wantCaretCol:  2,
		},
		{
			name:      "caret variable not found",
			envVars:   map[string]string{},
			input:     "SELECT $MISSING",
			wantErr:   true,
			caretLine: 1,
			caretCol:  10,
		},
		{
			name:           "file ref",
			workspaceFiles: map[string]string{"cte.sql": "SELECT id, name FROM users"},
			input:          "WITH ($cte) AS x SELECT * FROM x",
			wantSQL:        "WITH (SELECT id, name FROM users) AS x SELECT * FROM x",
			caretLine:      -1,
		},
		{
			name:           "file ref recursive",
			workspaceFiles: map[string]string{"base.sql": "SELECT id FROM t", "cte.sql": "SELECT * FROM ($base) b"},
			input:          "WITH ($cte) AS x SELECT * FROM x",
			wantSQL:        "WITH (SELECT * FROM (SELECT id FROM t) b) AS x SELECT * FROM x",
			caretLine:      -1,
		},
		{
			name:           "file ref safe break (cycle)",
			workspaceFiles: map[string]string{"cycle_a.sql": "SELECT * FROM ($cycle_b)", "cycle_b.sql": "SELECT * FROM ($cycle_a)"},
			input:          "WITH ($cycle_a) AS x SELECT 1",
			wantSQL:        "WITH (SELECT * FROM (SELECT * FROM ($cycle_a))) AS x SELECT 1",
			caretLine:      -1,
		},
		{
			name:           "file ref with env in file",
			workspaceFiles: map[string]string{"cte.sql": "SELECT $LIMIT"},
			envVars:        map[string]string{"LIMIT": "10"},
			input:          "WITH ($cte) AS x SELECT * FROM x",
			wantSQL:        "WITH (SELECT 10) AS x SELECT * FROM x",
			caretLine:      -1,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var g *graph.Graph
			var folderID string

			if c.workspaceFiles != nil {
				workspaceRoot, cleanup := createTempWorkspaceWithSQLFiles(t, c.workspaceFiles)
				defer cleanup()
				g = buildGraphFromTempWorkspace(t, "ws-subst", workspaceRoot)
				folderID = g.WorkspaceGraph.Folders[0].ID
				if len(c.envVars) > 0 {
					g.WorkspaceGraph.Folders[0].Variables = c.envVars
				}
			} else {
				g = graphWithEnvVars(t, c.envVars)
				folderID = "root"
			}

			if c.caretLine >= 0 {
				gotSQL, gotLine, gotCol, err := SubstituteVariablesSQLWithCaret(g, c.input, folderID, c.caretLine, c.caretCol)
				if c.wantErr {
					if err == nil {
						t.Fatalf("expected error, got nil")
					}
					if c.wantErrSubstr != "" && !strings.Contains(err.Error(), c.wantErrSubstr) {
						t.Errorf("error %q does not contain %q", err.Error(), c.wantErrSubstr)
					}
					return
				}
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if gotSQL != c.wantSQL {
					t.Errorf("sql: got %q, want %q", gotSQL, c.wantSQL)
				}
				if gotLine != c.wantCaretLine || gotCol != c.wantCaretCol {
					t.Errorf("caret: got (%d, %d), want (%d, %d)", gotLine, gotCol, c.wantCaretLine, c.wantCaretCol)
				}
				return
			}

			gotSQL, err := SubstituteVariablesSQL(g, c.input, folderID)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if c.wantErrSubstr != "" && !strings.Contains(err.Error(), c.wantErrSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), c.wantErrSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotSQL != c.wantSQL {
				t.Errorf("got %q, want %q", gotSQL, c.wantSQL)
			}
		})
	}
}

func graphWithEnvVars(t *testing.T, vars map[string]string) *graph.Graph {
	t.Helper()
	if vars == nil {
		vars = make(map[string]string)
	}
	return &graph.Graph{
		WorkspaceGraph: &graph.WorkspaceNode{
			ID: "ws-1",
			Folders: []*graph.FolderNode{
				{
					ID:        "root",
					URI:       "selectdb://workspaces/ws-1",
					FolderID:  "",
					Variables: vars,
					Files:     []*graph.FileNode{},
				},
			},
		},
	}
}

func createTempWorkspaceWithSQLFiles(t *testing.T, files map[string]string) (string, func()) {
	t.Helper()
	_, restore := withTempAppDataDirForSubstitution(t)
	workspaceID := "ws-subst"
	serverRoot, err := server.CurrentServerRoot()
	if err != nil {
		t.Fatalf("CurrentServerRoot: %v", err)
	}
	workspaceRoot := filepath.Join(serverRoot, "workspaces", workspaceID)
	if err := os.MkdirAll(workspaceRoot, 0o700); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(workspaceRoot, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return workspaceRoot, restore
}

func withTempAppDataDirForSubstitution(t *testing.T) (string, func()) {
	t.Helper()
	oldHome := os.Getenv("HOME")
	oldEnv := os.Getenv("APP_ENV")
	tempHome := t.TempDir()
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatalf("set HOME: %v", err)
	}
	if err := os.Setenv("APP_ENV", "test-substitution"); err != nil {
		t.Fatalf("set APP_ENV: %v", err)
	}
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
		t.Fatalf("write current domain: %v", err)
	}
	restore := func() {
		_ = os.Setenv("HOME", oldHome)
		_ = os.Setenv("APP_ENV", oldEnv)
	}
	return appRoot, restore
}

func buildGraphFromTempWorkspace(t *testing.T, workspaceID, workspaceRoot string) *graph.Graph {
	t.Helper()
	g := &graph.Graph{
		WorkspaceGraph: &graph.WorkspaceNode{
			ID:      workspaceID,
			Type:    "workspace",
			Name:    "Test",
			Folders: []*graph.FolderNode{},
		},
	}
	fsCtx := graph.NewWorkspaceFSFromRoot(workspaceID, workspaceRoot)
	if err := g.BuildWorkspaceGraphFromFS(fsCtx); err != nil {
		t.Fatalf("BuildWorkspaceGraphFromFS: %v", err)
	}
	return g
}
