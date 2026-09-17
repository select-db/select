package tokenanalyzer

// Severity represents the lint diagnostic severity level.
type Severity int

const (
	SeverityError   Severity = iota // SQL will not run correctly
	SeverityWarning                 // Anti-patterns and problematic constructs
	SeverityHint                    // Code style issues
)

// severityNames is the single table behind String and ParseSeverity. The
// strings are the values a .lint file accepts.
var severityNames = [...]string{
	SeverityError:   "error",
	SeverityWarning: "warning",
	SeverityHint:    "hint",
}

func (s Severity) String() string {
	if s < 0 || int(s) >= len(severityNames) {
		return "unknown"
	}
	return severityNames[s]
}

// ParseSeverity maps a .lint severity string to a Severity. An unrecognised
// name is a warning rather than an error, so a typo cannot silence a rule.
func ParseSeverity(name string) Severity {
	for severity, candidate := range severityNames {
		if candidate == name {
			return Severity(severity)
		}
	}
	return SeverityWarning
}

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
