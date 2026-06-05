package graph

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadEnvFile(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected map[string]string
	}{
		{
			name: "simple variables",
			content: `DB_HOST=localhost
DB_PORT=5432
DB_NAME=mydb`,
			expected: map[string]string{
				"DB_HOST": "localhost",
				"DB_PORT": "5432",
				"DB_NAME": "mydb",
			},
		},
		{
			name: "quoted values",
			content: `DB_USER="admin"
DB_PASS='secret123'
DB_DESC="My Database"`,
			expected: map[string]string{
				"DB_USER": "admin",
				"DB_PASS": "secret123",
				"DB_DESC": "My Database",
			},
		},
		{
			name: "comments and empty lines",
			content: `# Database configuration
DB_HOST=localhost

# Port number
DB_PORT=5432`,
			expected: map[string]string{
				"DB_HOST": "localhost",
				"DB_PORT": "5432",
			},
		},
		{
			name: "escape sequences",
			content: `MESSAGE="Hello\nWorld"
PATH="C:\\Users\\test"`,
			expected: map[string]string{
				"MESSAGE": "Hello\nWorld",
				"PATH":    "C:\\Users\\test",
			},
		},
		{
			name: "inline comments",
			content: `DB_HOST=localhost # production host
DB_PORT=5432`,
			expected: map[string]string{
				"DB_HOST": "localhost",
				"DB_PORT": "5432",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			envPath := filepath.Join(tmpDir, ".env")
			if err := os.WriteFile(envPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to create test .env file: %v", err)
			}

			vars, err := ReadEnvFile(envPath)
			if err != nil {
				t.Fatalf("ReadEnvFile() error = %v", err)
			}

			if len(vars) != len(tt.expected) {
				t.Errorf("Expected %d variables, got %d", len(tt.expected), len(vars))
			}

			for key, expectedValue := range tt.expected {
				if actualValue, ok := vars[key]; !ok {
					t.Errorf("Missing variable %s", key)
				} else if actualValue != expectedValue {
					t.Errorf("Variable %s: expected %q, got %q", key, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestReadEnvFile_NonExistent(t *testing.T) {
	vars, err := ReadEnvFile("/nonexistent/path/.env")
	if err != nil {
		t.Errorf("Expected no error for nonexistent file, got %v", err)
	}
	if len(vars) != 0 {
		t.Errorf("Expected empty map for nonexistent file, got %d variables", len(vars))
	}
}
