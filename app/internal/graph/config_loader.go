package graph

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

type Keybinding struct {
	Key     string `json:"key"`
	Command string `json:"command"`
	When    string `json:"when,omitempty"`
}

type KeybindingsCategories map[string][]Keybinding

type EditorSnippet struct {
	Prefix      string `json:"prefix"`
	Body        string `json:"body"`
	Description string `json:"description,omitempty"`
}

type Config struct {
	Keybindings    KeybindingsCategories `json:"keybindings"`
	EditorSnippets []EditorSnippet       `json:"editor_snippets"`
}

type ConfigResponse struct {
	Keybindings    []Keybinding    `json:"keybindings"`
	EditorSnippets []EditorSnippet `json:"editor_snippets"`
}

// Category order for flattening: workbench has precedence, then editor, modal, menu.
var keybindingCategoryOrder = []string{"workbench", "editor", "modal", "menu"}

// Converts the category-based config to a flat array for runtime matching.
// Categories are emitted in keybindingCategoryOrder so workbench bindings are matched first.
func (c KeybindingsCategories) Flatten() []Keybinding {
	var result []Keybinding
	seen := make(map[string]bool)
	for _, cat := range keybindingCategoryOrder {
		seen[cat] = true
		if bindings, ok := c[cat]; ok {
			result = append(result, bindings...)
		}
	}
	for cat, bindings := range c {
		if !seen[cat] {
			result = append(result, bindings...)
		}
	}
	return result
}

// Parses JSON config content into Config.
func parseConfig(content string) (*Config, error) {
	if content == "" {
		return &Config{
			Keybindings:    KeybindingsCategories{},
			EditorSnippets: []EditorSnippet{},
		}, nil
	}

	var config Config
	if err := json.Unmarshal([]byte(content), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if config.Keybindings == nil {
		config.Keybindings = KeybindingsCategories{}
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
				Keybindings:    KeybindingsCategories{},
				EditorSnippets: []EditorSnippet{},
			}, nil
		}
		return nil, fmt.Errorf("failed to read .config file: %w", err)
	}
	return parseConfig(string(data))
}

// Merges user overrides on top of defaults per category.
// User bindings with the same key+when combination replace defaults within each category.
// User bindings with unique key+when combinations are added.
// New categories from overrides are added entirely.
// Editor snippets: defaults first, then overrides; same prefix in overrides replaces the default.
func mergeConfig(defaults, overrides *Config) *Config {
	result := &Config{
		Keybindings:    make(KeybindingsCategories),
		EditorSnippets: mergeEditorSnippets(defaults.EditorSnippets, overrides.EditorSnippets),
	}

	// Process all categories from defaults
	for category, defaultBindings := range defaults.Keybindings {
		overrideBindings, hasOverrides := overrides.Keybindings[category]
		if !hasOverrides {
			result.Keybindings[category] = defaultBindings
			continue
		}

		// Build a map of override keys for quick lookup
		overrideMap := make(map[string]Keybinding)
		for _, kb := range overrideBindings {
			key := kb.Key + "|" + kb.When
			overrideMap[key] = kb
		}

		// Merge: defaults with overrides replacing where they match
		merged := make([]Keybinding, 0, len(defaultBindings)+len(overrideBindings))
		usedOverrides := make(map[string]bool)

		for _, kb := range defaultBindings {
			key := kb.Key + "|" + kb.When
			if override, exists := overrideMap[key]; exists {
				merged = append(merged, override)
				usedOverrides[key] = true
			} else {
				merged = append(merged, kb)
			}
		}

		// Add any overrides that weren't replacements (new bindings in category)
		for _, kb := range overrideBindings {
			key := kb.Key + "|" + kb.When
			if !usedOverrides[key] {
				merged = append(merged, kb)
			}
		}

		result.Keybindings[category] = merged
	}

	// Add any new categories from overrides that don't exist in defaults
	for category, bindings := range overrides.Keybindings {
		if _, exists := defaults.Keybindings[category]; !exists {
			result.Keybindings[category] = bindings
		}
	}

	return result
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

// LoadConfig returns the merged personal config: built-in defaults overlaid with
// the per-user .config (keybindings and editor snippets).
func (g *Graph) LoadConfig() (*ConfigResponse, error) {
	defaults, err := parseConfig(DefaultUserConfigContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse default user config: %w", err)
	}

	merged := defaults
	if userPath, err := GetUserConfigFilePath(); err == nil {
		userConfig, err := ReadConfigFile(userPath)
		if err != nil {
			return nil, err
		}
		merged = mergeConfig(defaults, userConfig)
	}

	return &ConfigResponse{
		Keybindings:    merged.Keybindings.Flatten(),
		EditorSnippets: merged.EditorSnippets,
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
