package graph

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadEnvFile reads and parses a .env file, returning a map of environment variables.
// Supports the full .env specification including:
// - Comments (lines starting with #)
// - Multi-line values (using quotes)
// - Single and double quoted values
// - Escaped characters
func ReadEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, fmt.Errorf("failed to open .env file: %w", err)
	}
	defer func() { _ = file.Close() }()

	vars := make(map[string]string)
	scanner := bufio.NewScanner(file)
	var multilineKey string
	var multilineValue strings.Builder
	var multilineQuote rune

	for scanner.Scan() {
		line := scanner.Text()

		if multilineKey != "" {
			if strings.HasSuffix(line, string(multilineQuote)) {
				multilineValue.WriteString("\n")
				multilineValue.WriteString(line[:len(line)-1])
				vars[multilineKey] = unescapeValue(multilineValue.String())
				multilineKey = ""
				multilineValue.Reset()
				multilineQuote = 0
			} else {
				multilineValue.WriteString("\n")
				multilineValue.WriteString(line)
			}
			continue
		}

		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		eqIdx := strings.Index(line, "=")
		if eqIdx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:eqIdx])
		valueRaw := strings.TrimSpace(line[eqIdx+1:])

		if !isValidEnvKey(key) {
			continue
		}

		value, isMultiline, quote := parseEnvValue(valueRaw)

		if isMultiline {
			multilineKey = key
			multilineValue.WriteString(value)
			multilineQuote = quote
		} else {
			vars[key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading .env file: %w", err)
	}

	return vars, nil
}

// isValidEnvKey reports whether key matches [A-Za-z_][A-Za-z0-9_]*.
func isValidEnvKey(key string) bool {
	if len(key) == 0 {
		return false
	}
	for i, r := range key {
		if i == 0 {
			if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && r != '_' {
				return false
			}
		} else {
			if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
				return false
			}
		}
	}
	return true
}

// parseEnvValue parses an environment variable value, handling quotes.
// Returns (value, isMultiline, quoteChar).
func parseEnvValue(raw string) (string, bool, rune) {
	if raw == "" {
		return "", false, 0
	}

	if strings.HasPrefix(raw, `"`) || strings.HasPrefix(raw, `'`) {
		quote := rune(raw[0])
		if len(raw) > 1 && strings.HasSuffix(raw, string(quote)) {
			return unescapeValue(raw[1 : len(raw)-1]), false, 0
		}
		return raw[1:], true, quote
	}

	if idx := strings.Index(raw, "#"); idx != -1 {
		raw = strings.TrimSpace(raw[:idx])
	}

	return unescapeValue(raw), false, 0
}

// unescapeValue processes escape sequences in environment variable values.
func unescapeValue(s string) string {
	var result strings.Builder
	result.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			next := s[i+1]
			switch next {
			case 'n':
				result.WriteByte('\n')
				i++
			case 'r':
				result.WriteByte('\r')
				i++
			case 't':
				result.WriteByte('\t')
				i++
			case '\\':
				result.WriteByte('\\')
				i++
			case '"':
				result.WriteByte('"')
				i++
			case '\'':
				result.WriteByte('\'')
				i++
			default:
				result.WriteByte(s[i])
			}
		} else {
			result.WriteByte(s[i])
		}
	}
	return result.String()
}

// ResolveVariable resolves a $varName reference: env vars walk the folder tree (precedence),
// SQL file refs use same-folder only.
// Implements sqllang.VarReplacer.
func (g *Graph) ResolveVariable(varName string, folderID string) (value string, isSqlFile bool, err error) {
	if _, err := g.GetWorkspaceGraph(); err != nil {
		return "", false, fmt.Errorf("workspace graph not initialized: %w", err)
	}

	currentFolderID := folderID
	visited := make(map[string]bool)

	for currentFolderID != "" {
		if visited[currentFolderID] {
			break
		}
		visited[currentFolderID] = true

		folder := g.GetFolderNodeByID(currentFolderID)
		if folder == nil {
			break
		}

		if folder.Variables != nil {
			if v, exists := folder.Variables[varName]; exists {
				return v, false, nil
			}
		}

		parentIDs := folder.GetParentIDs()
		if len(parentIDs) == 0 || parentIDs[0] == "" {
			break
		}
		currentFolderID = parentIDs[0]
	}

	content, readErr := g.readSqlFileContentByRefName(folderID, varName)
	if readErr != nil {
		return "", false, fmt.Errorf("variable $%s not found", varName)
	}
	return content, true, nil
}

// readSqlFileContentByRefName finds a .sql file in the given folder whose name (without extension)
// equals refName, and returns its content. Same-folder only.
func (g *Graph) readSqlFileContentByRefName(folderID string, refName string) (string, error) {
	wsGraph, err := g.GetWorkspaceGraph()
	if err != nil {
		return "", err
	}

	files, err := g.sqlFilesInFolder(folderID)
	if err != nil {
		return "", err
	}

	wfs, err := NewWorkspaceFS(wsGraph.ID)
	if err != nil {
		return "", err
	}

	for _, f := range files {
		if strings.TrimSuffix(f.Name, ".sql") != refName {
			continue
		}

		path, ok := wfs.Path(f.URI)
		if !ok {
			return "", fmt.Errorf("SQL file %s is not in this workspace", f.Name)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("reading SQL file %s: %w", f.Name, err)
		}
		return string(data), nil
	}
	return "", fmt.Errorf("no SQL file %s.sql in folder", refName)
}

// LoadFolderEnvFile loads the .env file for a folder and updates its Variables map.
func (g *Graph) LoadFolderEnvFile(folderNode *FolderNode, wfs *WorkspaceFS) error {
	folderPath, ok := wfs.Path(folderNode.URI)
	if !ok {
		return fmt.Errorf("folder %s is not in this workspace", folderNode.URI)
	}

	vars, err := ReadEnvFile(filepath.Join(folderPath, ".env"))
	if err != nil {
		return err
	}

	folderNode.Variables = vars
	return nil
}

// GetEnvFilePath returns the path to the .env file for a given folder URI.
func (g *Graph) GetEnvFilePath(folderURI string) (string, error) {
	wsGraph, err := g.GetWorkspaceGraph()
	if err != nil {
		return "", fmt.Errorf("workspace graph not initialized: %w", err)
	}

	wfs, err := NewWorkspaceFS(wsGraph.ID)
	if err != nil {
		return "", fmt.Errorf("failed to create workspace fs: %w", err)
	}

	folderPath, ok := wfs.Path(folderURI)
	if !ok {
		return "", fmt.Errorf("folder %s is not in this workspace", folderURI)
	}

	return filepath.Join(folderPath, ".env"), nil
}
