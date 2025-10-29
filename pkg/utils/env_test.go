package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileEnv(t *testing.T) {
	tests := []struct {
		name        string
		envKey      string
		envValue    string
		fileContent string
		defaultVal  string
		expectValue string
		expectError bool
	}{
		{
			name:        "EnvVarOnly",
			envKey:      "TEST_VAR",
			envValue:    "env_value",
			defaultVal:  "default",
			expectValue: "env_value",
			expectError: false,
		},
		{
			name:        "FileOnly",
			envKey:      "TEST_VAR",
			fileContent: "file_value\n",
			defaultVal:  "default",
			expectValue: "file_value",
			expectError: false,
		},
		{
			name:        "DefaultValue",
			envKey:      "TEST_VAR",
			defaultVal:  "default",
			expectValue: "default",
			expectError: false,
		},
		{
			name:        "BothSet",
			envKey:      "TEST_VAR",
			envValue:    "env_value",
			fileContent: "file_value",
			defaultVal:  "default",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv(tt.envKey)
			os.Unsetenv(tt.envKey + "_FILE")

			if tt.envValue != "" {
				os.Setenv(tt.envKey, tt.envValue)
				defer os.Unsetenv(tt.envKey)
			}

			if tt.fileContent != "" {
				tmpFile := filepath.Join(t.TempDir(), "secret")
				if err := os.WriteFile(tmpFile, []byte(tt.fileContent), 0600); err != nil {
					t.Fatal(err)
				}
				os.Setenv(tt.envKey+"_FILE", tmpFile)
				defer os.Unsetenv(tt.envKey + "_FILE")
			}

			result, err := FileEnv(tt.envKey, tt.defaultVal)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expectValue {
					t.Errorf("Expected %q, got %q", tt.expectValue, result)
				}
			}
		})
	}
}

func TestFileEnvNonExistentFile(t *testing.T) {
	os.Unsetenv("TEST_VAR")
	os.Unsetenv("TEST_VAR_FILE")

	os.Setenv("TEST_VAR_FILE", "/nonexistent/file")
	defer os.Unsetenv("TEST_VAR_FILE")

	_, err := FileEnv("TEST_VAR", "default")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}
