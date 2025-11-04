package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureIncludeDirective(t *testing.T) {
	tests := []struct {
		name            string
		initialContent  string
		includeFile     string
		expectedContent string
		shouldContain   string
	}{
		{
			name: "add include to empty file",
			initialContent: `# PostgreSQL configuration file
listen_addresses = '*'
port = 5432
`,
			includeFile:   "postgresql.tune.conf",
			shouldContain: "include_if_exists 'postgresql.tune.conf'",
		},
		{
			name: "include already exists",
			initialContent: `# PostgreSQL configuration file
listen_addresses = '*'
port = 5432
include_if_exists 'postgresql.tune.conf'
`,
			includeFile:   "postgresql.tune.conf",
			shouldContain: "include_if_exists 'postgresql.tune.conf'",
		},
		{
			name: "include with different syntax already exists",
			initialContent: `# PostgreSQL configuration file
listen_addresses = '*'
include 'postgresql.tune.conf'
port = 5432
`,
			includeFile:   "postgresql.tune.conf",
			shouldContain: "include 'postgresql.tune.conf'",
		},
		{
			name: "commented include should add new one",
			initialContent: `# PostgreSQL configuration file
listen_addresses = '*'
# include 'postgresql.tune.conf'
port = 5432
`,
			includeFile:   "postgresql.tune.conf",
			shouldContain: "include_if_exists 'postgresql.tune.conf'",
		},
		{
			name:           "file without trailing newline",
			initialContent: `listen_addresses = '*'`,
			includeFile:    "postgresql.tune.conf",
			shouldContain:  "include_if_exists 'postgresql.tune.conf'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			confPath := filepath.Join(tmpDir, "postgresql.conf")

			if err := os.WriteFile(confPath, []byte(tt.initialContent), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			err := EnsureIncludeDirective(confPath, tt.includeFile)
			if err != nil {
				t.Fatalf("EnsureIncludeDirective() error = %v", err)
			}

			resultContent, err := os.ReadFile(confPath)
			if err != nil {
				t.Fatalf("failed to read result file: %v", err)
			}

			resultStr := string(resultContent)

			if !strings.Contains(resultStr, tt.shouldContain) {
				t.Errorf("Result does not contain expected string.\nExpected to contain: %s\nGot:\n%s", tt.shouldContain, resultStr)
			}

			if tt.name == "include already exists" || tt.name == "include with different syntax already exists" {
				count := strings.Count(resultStr, "include")
				if count != 1 {
					t.Errorf("Expected exactly 1 include directive, got %d\nContent:\n%s", count, resultStr)
				}
			}
		})
	}
}

func TestEnsureIncludeDirective_FileNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "nonexistent.conf")

	err := EnsureIncludeDirective(confPath, "postgresql.tune.conf")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}
