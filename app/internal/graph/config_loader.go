package graph

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"selectDb/internal/keymap"
)

// DefaultUserConfigContent is the template for the per-user .config file. The
// .config file is personal: it owns keybindings and editor snippets only.
// Execution limits are workspace policy and live on the workspace row (see
// execution_limits.go), not in any file.
//
//go:embed defaults/user/.config
var DefaultUserConfigContent string

const ConfigFileName = ".config"

// GetDefaultUserConfigContent returns the default user .config content
// (keybindings and editor snippets). Used as the template/reset target.
func GetDefaultUserConfigContent() string {
	return DefaultUserConfigContent
}

type EditorSnippet struct {
	Prefix      string `json:"prefix"`
	Body        string `json:"body"`
	Description string `json:"description,omitempty"`
}

// Config is a .config file as it is written: keybindings grouped by the part
// of the app they belong to, and editor snippets. Parsing a binding and
// deciding which one wins is internal/keymap's; this package reads the files.
type Config struct {
	Keybindings    keymap.Categories `json:"keybindings"`
	EditorSnippets []EditorSnippet   `json:"editor_snippets"`
}

// ConfigResponse is the personal config as the app uses it: keybindings with
// their chords parsed and resolved for this platform, in the order they are
// matched, plus whatever was wrong with the ones that could not be.
type ConfigResponse struct {
	Keybindings    []keymap.Binding `json:"keybindings"`
	EditorSnippets []EditorSnippet  `json:"editor_snippets"`
	Problems       []keymap.Problem `json:"problems"`
	OS             string           `json:"os"`
}

// Parses JSON config content into Config.
func parseConfig(content string) (*Config, error) {
	if content == "" {
		return &Config{
			Keybindings:    keymap.Categories{},
			EditorSnippets: []EditorSnippet{},
		}, nil
	}

	var config Config
	if err := json.Unmarshal([]byte(content), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if config.Keybindings == nil {
		config.Keybindings = keymap.Categories{}
	}
	if config.EditorSnippets == nil {
		config.EditorSnippets = []EditorSnippet{}
	}

	return &config, nil
}

// Reads and parses a .config file.
func ReadConfigFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{
				Keybindings:    keymap.Categories{},
				EditorSnippets: []EditorSnippet{},
			}, nil
		}
		return nil, fmt.Errorf("failed to read .config file: %w", err)
	}
	return parseConfig(string(data))
}

// mergeEditorSnippets merges default and user snippets. User snippet with same prefix overrides.
func mergeEditorSnippets(defaults, overrides []EditorSnippet) []EditorSnippet {
	byPrefix := make(map[string]EditorSnippet)
	for _, s := range defaults {
		byPrefix[s.Prefix] = s
	}
	for _, s := range overrides {
		byPrefix[s.Prefix] = s
	}
	// Deterministic order: defaults first (overwritten by overrides), then overrides-only prefixes
	seen := make(map[string]bool)
	var out []EditorSnippet
	for _, s := range defaults {
		snippet := byPrefix[s.Prefix]
		out = append(out, snippet)
		seen[s.Prefix] = true
	}
	for _, s := range overrides {
		if !seen[s.Prefix] {
			out = append(out, s)
		}
	}
	return out
}

// GetUserConfigFilePath returns the path to the per-user .config file (personal
// keybindings and editor snippets), which lives outside every workspace.
func GetUserConfigFilePath() (string, error) {
	dir, err := UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ConfigFileName), nil
}

// LoadConfig returns the personal config the app runs on: the built-in
// defaults, then the per-user .config, with every chord parsed and resolved for
// this platform and the bindings in the order they are matched.
func (g *Graph) LoadConfig() (*ConfigResponse, error) {
	defaults, err := parseConfig(DefaultUserConfigContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse default user config: %w", err)
	}

	user := &Config{Keybindings: keymap.Categories{}, EditorSnippets: []EditorSnippet{}}
	userPath, pathErr := GetUserConfigFilePath()
	if pathErr == nil {
		user, err = ReadConfigFile(userPath)
		if err != nil {
			return nil, err
		}
	}

	platform := keymap.Current()
	bindings, problems := keymap.Resolve(defaults.Keybindings, user.Keybindings, platform)

	return &ConfigResponse{
		Keybindings:    bindings,
		EditorSnippets: mergeEditorSnippets(defaults.EditorSnippets, user.EditorSnippets),
		Problems:       problems,
		OS:             string(platform),
	}, nil
}

// ResetUserConfig writes the default keybindings/snippets content to the
// per-user .config file.
func ResetUserConfig() error {
	configPath, err := GetUserConfigFilePath()
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, []byte(DefaultUserConfigContent), 0644)
}
