package graph

import (
	"os"
	"path/filepath"
	"strings"
)

// SqlFilePreviewLen is the max number of characters used for SQL file preview in the UI.
const SqlFilePreviewLen = 80

// VariableCandidate represents a single environment variable with its value and source
type VariableCandidate struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	Source    string `json:"source"`     // The folder name where this variable is defined
	SourceURI string `json:"source_uri"` // URI of the .env file where this variable is defined
}

// SqlFileCandidate represents a SQL file in the same folder that can be referenced as $name
type SqlFileCandidate struct {
	Name    string `json:"name"`    // file name without .sql
	Path    string `json:"path"`    // relative path from workspace root for display
	URI     string `json:"uri"`     // full URI
	Preview string `json:"preview"` // first SqlFilePreviewLen chars of file content
}

// GetUriVariables returns all available environment variables for a given URI,
// walking up the folder hierarchy to collect variables. Child folder variables
// take precedence over parent folder variables.
func (g *Graph) GetUriVariables(uri string) ([]VariableCandidate, error) {
	// Get the workspace graph
	if _, err := g.GetWorkspaceGraph(); err != nil {
		return nil, err
	}

	// Find the node to get its folder ID
	var folderID string
	switch node := g.nodeForURI(uri).(type) {
	case *FileNode:
		folderID = node.FolderID
	case *FolderNode:
		folderID = node.ID
	case *DBInstanceNode:
		folderID = node.FolderID
	default:
		return []VariableCandidate{}, nil
	}

	// Collect variables from the folder hierarchy
	variables := make(map[string]VariableCandidate)
	visited := make(map[string]bool)
	currentFolderID := folderID

	// Walk up the folder tree, collecting variables
	// Variables from child folders take precedence (added first)
	for currentFolderID != "" {
		// Prevent infinite loops
		if visited[currentFolderID] {
			break
		}
		visited[currentFolderID] = true

		// Find folder node
		folder := g.GetFolderNodeByID(currentFolderID)
		if folder == nil {
			break
		}

		// Add variables from this folder (only if not already present)
		if folder.Variables != nil {
			for key, value := range folder.Variables {
				if _, exists := variables[key]; !exists {
					variables[key] = VariableCandidate{
						Name:      key,
						Value:     value,
						Source:    folder.Name,
						SourceURI: folder.URI + "/.env",
					}
				}
			}
		}

		// Move to parent folder
		parentIDs := folder.GetParentIDs()
		if len(parentIDs) == 0 || parentIDs[0] == "" {
			break
		}
		currentFolderID = parentIDs[0]
	}

	// Convert map to sorted slice
	result := make([]VariableCandidate, 0, len(variables))
	for _, v := range variables {
		result = append(result, v)
	}

	// Sort by name for consistent ordering
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].Name > result[j].Name {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result, nil
}

// GetUriSqlFileRefs returns SQL files in the same folder as the given URI (same-folder only).
// Each candidate includes a short preview of the file content for the suggestion menu.
func (g *Graph) GetUriSqlFileRefs(uri string) ([]SqlFileCandidate, error) {
	wsGraph, err := g.GetWorkspaceGraph()
	if err != nil {
		return nil, err
	}

	var folderID string
	switch node := g.nodeForURI(uri).(type) {
	case *FileNode:
		folderID = node.FolderID
	case *FolderNode:
		folderID = node.ID
	case *DBInstanceNode:
		folderID = node.FolderID
	default:
		return []SqlFileCandidate{}, nil
	}

	// The candidates are the folder's own .sql files, so it has to have been
	// read from disk.
	folder := g.folderWithFiles(folderID)
	if folder == nil {
		return []SqlFileCandidate{}, nil
	}

	wfs, err := NewWorkspaceFS(wsGraph.ID)
	if err != nil {
		return nil, err
	}

	var result []SqlFileCandidate
	for _, f := range folder.Files {
		if !strings.HasSuffix(f.Name, ".sql") {
			continue
		}
		// Exclude the current file so it is not suggested for itself
		if f.URI == uri {
			continue
		}
		nameNoExt := strings.TrimSuffix(f.Name, ".sql")
		rel := strings.TrimPrefix(f.URI, wfs.RootURI+"/")
		if rel == f.URI {
			rel = f.URI
		}
		path := rel

		preview := ""
		fullPath := filepath.Join(wfs.WorkspaceRoot, filepath.FromSlash(rel))
		if data, err := os.ReadFile(fullPath); err == nil {
			preview = string(data)
			if len(preview) > SqlFilePreviewLen {
				preview = preview[:SqlFilePreviewLen] + "..."
			}
			preview = strings.TrimSpace(preview)
		}

		result = append(result, SqlFileCandidate{
			Name:    nameNoExt,
			Path:    path,
			URI:     f.URI,
			Preview: preview,
		})
	}

	// Sort by name
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].Name > result[j].Name {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result, nil
}
