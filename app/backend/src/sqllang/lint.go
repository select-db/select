package sqllang

import (
	"fmt"
	"strings"

	"selectDb/backend/src/role"
	"selectDb/backend/utils"

	core "github.com/selectDb/dialect/core"
	ta "github.com/selectDb/dialect/core/tokenanalyzer"
	"github.com/selectDb/dialect/engine"
)

// LintResult is the lint response returned to the frontend.
type LintResult struct {
	ID          string           `json:"id"`
	Diagnostics []LintDiagnostic `json:"diagnostics"`
	Errors      []string         `json:"errors"`
}

// LintDiagnostic is a serialisable diagnostic with Monaco-compatible severity.
type LintDiagnostic struct {
	RuleID    string `json:"ruleId"`
	Severity  int    `json:"severity"` // 8=Error, 4=Warning, 1=Hint (Monaco MarkerSeverity)
	Message   string `json:"message"`
	StartLine int    `json:"startLine"`
	StartCol  int    `json:"startCol"`
	EndLine   int    `json:"endLine"`
	EndCol    int    `json:"endCol"`
}

// Lint analyses the SQL in p and returns a list of diagnostics.
func (s *SqlLang) Lint(p PositionParams) LintResult {
	id, err := utils.GenerateRandomID(4)
	if err != nil {
		return LintResult{Errors: []string{"failed to generate result id"}}
	}

	dbInstance := s.graph.GetDBInstanceNodeByID(p.DbInstanceID)
	if dbInstance == nil {
		return LintResult{ID: id, Errors: []string{fmt.Sprintf("DB instance not found: %s", p.DbInstanceID)}}
	}
	meta, err := s.getMeta(dbInstance, false)
	if err != nil {
		return LintResult{ID: id, Errors: []string{fmt.Sprintf("failed to get metadata: %v", err)}}
	}

	sql, _, _ := s.resolveEditorPosition(p)

	dialect := engine.GetDialect(dbInstance.DBType)
	if dialect == nil {
		return LintResult{ID: id, Errors: []string{fmt.Sprintf("unsupported DB type: %s", dbInstance.DBType)}}
	}

	var filePath string
	if p.FileID != "" {
		if file := s.graph.GetFileNodeByID(p.FileID); file != nil {
			filePath = file.URI
		}
	}

	lintEntries, _ := s.graph.LoadWorkspaceLint()
	cfg := ta.ResolveLintConfig(lintEntries, filePath)

	diagnostics := s.lintRunner.Run(sql, dialect, *meta, cfg)

	out := LintResult{ID: id, Diagnostics: make([]LintDiagnostic, 0, len(diagnostics))}
	for _, diagnostic := range diagnostics {
		out.Diagnostics = append(out.Diagnostics, LintDiagnostic{
			RuleID:    diagnostic.RuleID,
			Severity:  severityToMonaco(diagnostic.Severity),
			Message:   diagnostic.Message,
			StartLine: diagnostic.StartLine,
			StartCol:  diagnostic.StartCol,
			EndLine:   diagnostic.EndLine,
			EndCol:    diagnostic.EndCol,
		})
	}

	permissions, err := role.GetMyPermissions(s.queries)
	if err != nil {
		return out
	}

	compiledPermissions := core.Compile(permissions)
	inspectStatements := s.inspect(sql, dbInstance)

	permissionError, ok := core.CheckQueryPermissions(inspectStatements, p.DbInstanceID, compiledPermissions).(*core.PermissionDeniedError)
	if !ok {
		return out
	}

	startLine, startCol, diagEndLine, endCol := permissionError.StartLine, permissionError.StartCol, permissionError.StartLine, permissionError.EndCol
	if startLine == 0 {
		startLine, startCol = 1, 0
		diagEndLine, endCol = lastLineCol(sql)
	}

	out.Diagnostics = append(out.Diagnostics, LintDiagnostic{
		RuleID:    core.RuleIDPermissionDenied,
		Severity:  8,
		Message:   permissionError.Error(),
		StartLine: startLine,
		StartCol:  startCol,
		EndLine:   diagEndLine,
		EndCol:    endCol,
	})

	return out
}

// lastLineCol returns the 1-based line number and 0-based column of the last character in sql.
func lastLineCol(sql string) (line, col int) {
	line = strings.Count(sql, "\n") + 1
	if last := strings.LastIndex(sql, "\n"); last >= 0 {
		col = len(sql) - last - 1
	} else {
		col = len(sql)
	}
	return line, col
}

// severityToMonaco maps lint.Severity to Monaco MarkerSeverity values.
func severityToMonaco(s ta.Severity) int {
	switch s {
	case ta.SeverityError:
		return 8 // MarkerSeverity.Error
	case ta.SeverityWarning:
		return 4 // MarkerSeverity.Warning
	default:
		return 1 // MarkerSeverity.Hint
	}
}
