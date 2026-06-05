package graph

import (
	"testing"
)

func TestGetUriVariables(t *testing.T) {
	// Create a mock graph with nested folders and variables
	rootFolder := &FolderNode{
		ID:       "root",
		URI:      "selectdb://workspaces/test",
		Type:     "folder",
		Name:     "root",
		FolderID: "",
		Variables: map[string]string{
			"ROOT_VAR":  "root_value",
			"DB_HOST":   "localhost",
			"SHARED":    "from_root",
			"API_KEY":   "root_api_key",
		},
	}

	childFolder := &FolderNode{
		ID:       "child",
		URI:      "selectdb://workspaces/test/child",
		Type:     "folder",
		Name:     "child",
		FolderID: "root",
		Variables: map[string]string{
			"CHILD_VAR": "child_value",
			"DB_HOST":   "production.example.com", // Override parent
			"SHARED":    "from_child",              // Override parent
		},
	}

	grandchildFolder := &FolderNode{
		ID:       "grandchild",
		URI:      "selectdb://workspaces/test/child/grandchild",
		Type:     "folder",
		Name:     "grandchild",
		FolderID: "child",
		Variables: map[string]string{
			"GRANDCHILD_VAR": "grandchild_value",
			"DB_HOST":        "dev.example.com", // Override parent and grandparent
		},
	}

	testFile := &FileNode{
		ID:       "file-1",
		URI:      "selectdb://workspaces/test/child/grandchild/query.sql",
		Type:     "file",
		Name:     "query.sql",
		FolderID: "grandchild",
	}

	// Build the tree structure
	grandchildFolder.Files = []*FileNode{testFile}
	childFolder.Folders = []*FolderNode{grandchildFolder}
	rootFolder.Folders = []*FolderNode{childFolder}

	g := &Graph{
		WorkspaceGraph: &WorkspaceNode{
			ID:      "ws-1",
			Type:    "workspace",
			Name:    "Test Workspace",
			Folders: []*FolderNode{rootFolder},
		},
	}

	tests := []struct {
		name     string
		uri      string
		expected map[string]VariableCandidate
		wantErr  bool
	}{
		{
			name: "file in grandchild folder",
			uri:  "file-1",
			expected: map[string]VariableCandidate{
				"GRANDCHILD_VAR": {Name: "GRANDCHILD_VAR", Value: "grandchild_value", Source: "grandchild"},
				"DB_HOST":        {Name: "DB_HOST", Value: "dev.example.com", Source: "grandchild"},
				"CHILD_VAR":      {Name: "CHILD_VAR", Value: "child_value", Source: "child"},
				"SHARED":         {Name: "SHARED", Value: "from_child", Source: "child"},
				"ROOT_VAR":       {Name: "ROOT_VAR", Value: "root_value", Source: "root"},
				"API_KEY":        {Name: "API_KEY", Value: "root_api_key", Source: "root"},
			},
			wantErr: false,
		},
		{
			name: "folder URI - child folder",
			uri:  "child",
			expected: map[string]VariableCandidate{
				"CHILD_VAR": {Name: "CHILD_VAR", Value: "child_value", Source: "child"},
				"DB_HOST":   {Name: "DB_HOST", Value: "production.example.com", Source: "child"},
				"SHARED":    {Name: "SHARED", Value: "from_child", Source: "child"},
				"ROOT_VAR":  {Name: "ROOT_VAR", Value: "root_value", Source: "root"},
				"API_KEY":   {Name: "API_KEY", Value: "root_api_key", Source: "root"},
			},
			wantErr: false,
		},
		{
			name: "root folder",
			uri:  "root",
			expected: map[string]VariableCandidate{
				"ROOT_VAR": {Name: "ROOT_VAR", Value: "root_value", Source: "root"},
				"DB_HOST":  {Name: "DB_HOST", Value: "localhost", Source: "root"},
				"SHARED":   {Name: "SHARED", Value: "from_root", Source: "root"},
				"API_KEY":  {Name: "API_KEY", Value: "root_api_key", Source: "root"},
			},
			wantErr: false,
		},
		{
			name:     "non-existent URI",
			uri:      "non-existent",
			expected: map[string]VariableCandidate{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := g.GetUriVariables(tt.uri)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Convert result to map for easier comparison
			resultMap := make(map[string]VariableCandidate)
			for _, v := range result {
				resultMap[v.Name] = v
			}

			// Check if all expected variables are present
			if len(resultMap) != len(tt.expected) {
				t.Errorf("Expected %d variables, got %d", len(tt.expected), len(resultMap))
			}

			for name, expectedVar := range tt.expected {
				actualVar, ok := resultMap[name]
				if !ok {
					t.Errorf("Missing variable %s", name)
					continue
				}

				if actualVar.Value != expectedVar.Value {
					t.Errorf("Variable %s: expected value %q, got %q", name, expectedVar.Value, actualVar.Value)
				}

				if actualVar.Source != expectedVar.Source {
					t.Errorf("Variable %s: expected source %q, got %q", name, expectedVar.Source, actualVar.Source)
				}
			}
		})
	}
}

func TestGetUriVariables_EmptyVariables(t *testing.T) {
	// Create a graph with folders but no variables
	folder := &FolderNode{
		ID:        "folder-1",
		URI:       "selectdb://workspaces/test/folder",
		Type:      "folder",
		Name:      "folder",
		FolderID:  "root",
		Variables: map[string]string{},
	}

	file := &FileNode{
		ID:       "file-1",
		URI:      "selectdb://workspaces/test/folder/query.sql",
		Type:     "file",
		Name:     "query.sql",
		FolderID: "folder-1",
	}

	folder.Files = []*FileNode{file}

	g := &Graph{
		WorkspaceGraph: &WorkspaceNode{
			ID:      "ws-1",
			Type:    "workspace",
			Name:    "Test Workspace",
			Folders: []*FolderNode{folder},
		},
	}

	result, err := g.GetUriVariables("file-1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected 0 variables, got %d", len(result))
	}
}

func TestGetUriVariables_DBInstance(t *testing.T) {
	// Test that GetUriVariables works for DBInstance nodes
	folder := &FolderNode{
		ID:       "folder-1",
		URI:      "selectdb://workspaces/test/folder",
		Type:     "folder",
		Name:     "folder",
		FolderID: "root",
		Variables: map[string]string{
			"DB_USER": "admin",
			"DB_PASS": "secret",
		},
	}

	dbInstance := &DBInstanceNode{
		ID:          "db-1",
		URI:         "selectdb://workspaces/test/folder/mydb",
		Type:        "db_instance",
		Name:        "mydb",
		FolderID:    "folder-1",
		WorkspaceID: "ws-1",
	}

	folder.DBInstances = []*DBInstanceNode{dbInstance}

	g := &Graph{
		WorkspaceGraph: &WorkspaceNode{
			ID:          "ws-1",
			Type:        "workspace",
			Name:        "Test Workspace",
			Folders:     []*FolderNode{folder},
			DBInstances: []*DBInstanceNode{dbInstance},
		},
	}

	result, err := g.GetUriVariables("db-1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 variables, got %d", len(result))
	}

	// Verify variables
	resultMap := make(map[string]string)
	for _, v := range result {
		resultMap[v.Name] = v.Value
	}

	if resultMap["DB_USER"] != "admin" {
		t.Errorf("Expected DB_USER=admin, got %s", resultMap["DB_USER"])
	}

	if resultMap["DB_PASS"] != "secret" {
		t.Errorf("Expected DB_PASS=secret, got %s", resultMap["DB_PASS"])
	}
}
