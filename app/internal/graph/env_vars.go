package graph

import (
	"context"
	"os"
	"slices"
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

	// The folder a URI's variables come from: its own, if it is a folder.
	folderID := g.folderIDForURI(uri)
	if folderID == "" {
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

	slices.SortFunc(result, func(a, b VariableCandidate) int { return strings.Compare(a.Name, b.Name) })

	return result, nil
}

// folderIDForURI returns the folder a URI belongs to — itself, when the URI is
// a folder — or "" when the graph has nothing under it.
func (g *Graph) folderIDForURI(uri string) string {
	switch node := g.nodeForURI(uri).(type) {
	case *FileNode:
		return node.FolderID
	case *FolderNode:
		return node.ID
	case *DBInstanceNode:
		return node.FolderID
	default:
		return ""
	}
}

// sqlFilesInFolder returns the .sql files a $ref can name: the folder's own,
// nothing deeper. They are asked for rather than read off the folder node, so a
// folder nobody has opened answers without being pulled into the graph.
func (g *Graph) sqlFilesInFolder(folderURI string) ([]*FileNode, error) {
	return g.FindFiles(context.Background(), FileQuery{
		FolderURI:  folderURI,
		Extensions: []string{".sql"},
		Depth:      1,
		Limit:      MaxFileQueryLimit,
	})
}

// GetUriSqlFileRefs returns SQL files in the same folder as the given URI (same-folder only).
// Each candidate includes a short preview of the file content for the suggestion menu.
func (g *Graph) GetUriSqlFileRefs(uri string) ([]SqlFileCandidate, error) {
	wsGraph, err := g.GetWorkspaceGraph()
	if err != nil {
		return nil, err
	}

	folderID := g.folderIDForURI(uri)
	if folderID == "" {
		return []SqlFileCandidate{}, nil
	}

	files, err := g.sqlFilesInFolder(folderID)
	if err != nil {
		return nil, err
	}

	wfs, err := NewWorkspaceFS(wsGraph.ID)
	if err != nil {
		return nil, err
	}

	var result []SqlFileCandidate
	for _, f := range files {
		// Exclude the current file so it is not suggested for itself
		if f.URI == uri {
			continue
		}
		fullPath, ok := wfs.Path(f.URI)
		if !ok {
			continue
		}

		preview := ""
		if data, err := os.ReadFile(fullPath); err == nil {
			preview = strings.TrimSpace(string(data))
			if len(preview) > SqlFilePreviewLen {
				preview = preview[:SqlFilePreviewLen] + "..."
			}
		}

		result = append(result, SqlFileCandidate{
			Name:    strings.TrimSuffix(f.Name, ".sql"),
			Path:    strings.TrimPrefix(f.URI, wfs.RootURI+"/"),
			URI:     f.URI,
			Preview: preview,
		})
	}

	slices.SortFunc(result, func(a, b SqlFileCandidate) int { return strings.Compare(a.Name, b.Name) })

	return result, nil
}
