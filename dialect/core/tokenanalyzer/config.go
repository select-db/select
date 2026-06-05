package tokenanalyzer

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// LintRuleConfig holds a severity override for a built-in rule.
// Accepts either a plain string ("off") or a tuple (["warning", {...}]) for forward compatibility.
type LintRuleConfig struct {
	Severity string
}

func (r *LintRuleConfig) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		r.Severity = s
		return nil
	}
	var tuple []json.RawMessage
	if err := json.Unmarshal(data, &tuple); err != nil || len(tuple) == 0 {
		return fmt.Errorf("rule config must be a severity string or [severity, options] tuple")
	}
	return json.Unmarshal(tuple[0], &r.Severity)
}

// LintConfigEntry is a single entry in the flat config array.
// Entries without Files apply globally. Ignores-only entries skip matching files entirely.
type LintConfigEntry struct {
	Files   []string                  `json:"files,omitempty"`
	Ignores []string                  `json:"ignores,omitempty"`
	Rules   map[string]LintRuleConfig `json:"rules,omitempty"`
	Custom  []CustomRule              `json:"custom,omitempty"`
}

// LintFile is the ordered list of config entries parsed from a .lint file.
type LintFile = []LintConfigEntry

// ResolvedLintConfig is the merged result of all entries applicable to a given file path.
type ResolvedLintConfig struct {
	Rules  map[string]LintRuleConfig
	Custom []CustomRule
}

// ParseLintConfig parses a .lint JSON string into a LintFile.
func ParseLintConfig(content string) (LintFile, error) {
	if content == "" {
		return LintFile{}, nil
	}
	var entries LintFile
	if err := json.Unmarshal([]byte(content), &entries); err != nil {
		return nil, fmt.Errorf("failed to parse .lint: %w", err)
	}
	return entries, nil
}

// ResolveLintConfig merges all applicable entries for filePath.
// Entries are applied top-to-bottom; later entries override earlier ones.
// filePath may be empty, in which case only global entries (no Files field) apply.
func ResolveLintConfig(entries LintFile, filePath string) ResolvedLintConfig {
	resolved := ResolvedLintConfig{
		Rules: map[string]LintRuleConfig{},
	}
	for _, entry := range entries {
		// Ignores-only entry: skip the file entirely.
		if len(entry.Ignores) > 0 && filePath != "" && matchesGlob(filePath, entry.Ignores) {
			return ResolvedLintConfig{Rules: map[string]LintRuleConfig{}}
		}
		// Skip entries scoped to files that don't match.
		if len(entry.Files) > 0 && (filePath == "" || !matchesGlob(filePath, entry.Files)) {
			continue
		}
		for k, v := range entry.Rules {
			resolved.Rules[k] = v
		}
		resolved.Custom = append(resolved.Custom, entry.Custom...)
	}
	return resolved
}

// matchesGlob reports whether filePath matches any of the given glob patterns.
// filePath may be absolute; patterns are matched against the full path, the
// basename, and every trailing sub-path so that "migrations/**" matches both
// "migrations/a.sql" and "/abs/workspace/migrations/a.sql".
func matchesGlob(filePath string, patterns []string) bool {
	normalised := filepath.ToSlash(filePath)
	name := filepath.Base(filePath)

	for _, pattern := range patterns {
		if ok, _ := filepath.Match(pattern, normalised); ok {
			return true
		}
		if ok, _ := filepath.Match(pattern, name); ok {
			return true
		}
		if strings.HasSuffix(pattern, "/**") {
			prefix := strings.TrimSuffix(pattern, "/**")
			if strings.Contains(normalised, "/"+prefix+"/") ||
				strings.HasPrefix(normalised, prefix+"/") {
				return true
			}
		}
	}
	return false
}
