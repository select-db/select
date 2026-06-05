package tokenanalyzer

// Severity represents the lint diagnostic severity level.
type Severity int

const (
	SeverityError   Severity = iota // SQL will not run correctly
	SeverityWarning                 // Anti-patterns and problematic constructs
	SeverityHint                    // Code style issues
)

// Diagnostic is a single lint finding with source location.
type Diagnostic struct {
	RuleID   string
	Severity Severity
	Message  string
	// Positions are 1-based line, 0-based column.
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int // exclusive
}
