package graph

import (
	"bufio"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// DefaultThemeContent is embedded from defaults/user/.theme.
// The theme is a personal (per-user) file, so its default lives under the user
// defaults.
//
//go:embed defaults/user/.theme
var DefaultThemeContent string

const ThemeFileName = ".theme"

// Matches :root { or :root[data-theme="light|dark"] {
var rootSelectorPattern = regexp.MustCompile(`^\s*:root(?:\[data-theme="(light|dark)"\])?\s*\{`)

// Matches --name: value; inside a block
var cssVarPattern = regexp.MustCompile(`^\s*(--[a-zA-Z0-9-]+)\s*:\s*(.+?)\s*;?\s*$`)

type ThemeVariables struct {
	Shared map[string]string `json:"shared"`
	Light  map[string]string `json:"light"`
	Dark   map[string]string `json:"dark"`
}

// parseTheme parses a .theme file (valid CSS) into ThemeVariables.
func parseTheme(content string) *ThemeVariables {
	result := &ThemeVariables{
		Shared: make(map[string]string),
		Light:  make(map[string]string),
		Dark:   make(map[string]string),
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	inBlockComment := false
	currentSection := ""

	for scanner.Scan() {
		line := parseLine(scanner.Text(), &inBlockComment)
		if line == "" {
			continue
		}

		// Detect opening selector
		if m := rootSelectorPattern.FindStringSubmatch(line); m != nil {
			switch m[1] {
			case "light":
				currentSection = "light"
			case "dark":
				currentSection = "dark"
			default:
				currentSection = "shared"
			}
			continue
		}

		// Detect closing brace
		if line == "}" {
			currentSection = ""
			continue
		}

		if currentSection == "" {
			continue
		}

		// Extract CSS variable
		if m := cssVarPattern.FindStringSubmatch(line); len(m) == 3 {
			name := m[1]
			value := strings.TrimSuffix(strings.TrimSpace(m[2]), ";")
			switch currentSection {
			case "light":
				result.Light[name] = value
			case "dark":
				result.Dark[name] = value
			default:
				result.Shared[name] = value
			}
		}
	}

	return result
}

// ReadThemeFile reads and parses a .theme file.
// Returns empty maps if the file doesn't exist.
func ReadThemeFile(path string) (*ThemeVariables, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ThemeVariables{
				Shared: make(map[string]string),
				Light:  make(map[string]string),
				Dark:   make(map[string]string),
			}, nil
		}
		return nil, fmt.Errorf("failed to read .theme file: %w", err)
	}
	return parseTheme(string(data)), nil
}

// parseLine strips comments from a single line.
func parseLine(line string, inBlockComment *bool) string {
	if *inBlockComment {
		if idx := strings.Index(line, "*/"); idx != -1 {
			*inBlockComment = false
			line = line[idx+2:]
		} else {
			return ""
		}
	}

	if idx := strings.Index(line, "/*"); idx != -1 {
		if end := strings.Index(line, "*/"); end != -1 {
			line = line[:idx] + line[end+2:]
		} else {
			*inBlockComment = true
			line = line[:idx]
		}
	}

	return strings.TrimSpace(line)
}

// mergeThemeVariables merges user overrides on top of defaults.
func mergeThemeVariables(defaults, overrides *ThemeVariables) *ThemeVariables {
	result := &ThemeVariables{
		Shared: make(map[string]string),
		Light:  make(map[string]string),
		Dark:   make(map[string]string),
	}
	for k, v := range defaults.Shared {
		result.Shared[k] = v
	}
	for k, v := range defaults.Light {
		result.Light[k] = v
	}
	for k, v := range defaults.Dark {
		result.Dark[k] = v
	}
	for k, v := range overrides.Shared {
		result.Shared[k] = v
	}
	for k, v := range overrides.Light {
		result.Light[k] = v
	}
	for k, v := range overrides.Dark {
		result.Dark[k] = v
	}
	return result
}

// GetThemeFilePath returns the path to the per-user .theme file. The theme is
// personal to the user and lives outside every workspace, so a team cannot
// dictate appearance through the shared repo.
func (g *Graph) GetThemeFilePath() (string, error) {
	return GetUserThemeFilePath()
}

// GetUserThemeFilePath returns the path to the per-user .theme file.
func GetUserThemeFilePath() (string, error) {
	dir, err := UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ThemeFileName), nil
}

// LoadWorkspaceTheme loads theme variables by merging the built-in defaults with
// the per-user .theme overrides. The workspace layer is intentionally dropped:
// appearance is the user's to own and follows them across every workspace.
func (g *Graph) LoadWorkspaceTheme() (*ThemeVariables, error) {
	themePath, err := GetUserThemeFilePath()
	if err != nil {
		return nil, err
	}
	defaults := parseTheme(DefaultThemeContent)
	userOverrides, err := ReadThemeFile(themePath)
	if err != nil {
		return nil, err
	}
	return mergeThemeVariables(defaults, userOverrides), nil
}

// ResetWorkspaceTheme writes the default theme content to the per-user .theme file.
func (g *Graph) ResetWorkspaceTheme() error {
	themePath, err := GetUserThemeFilePath()
	if err != nil {
		return err
	}
	return os.WriteFile(themePath, []byte(DefaultThemeContent), 0644)
}

// GetDefaultThemeContent returns the default theme file content.
func GetDefaultThemeContent() string {
	return DefaultThemeContent
}

// LoadDefaultTheme returns theme variables from the embedded default .theme file.
// Used when there is no workspace (e.g. login page) so the app still has a valid theme.
func LoadDefaultTheme() *ThemeVariables {
	return parseTheme(DefaultThemeContent)
}
