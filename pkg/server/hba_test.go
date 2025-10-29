package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/flanksource/postgres/pkg"
)

func TestSetupPgHBA(t *testing.T) {
	tests := []struct {
		name           string
		authMethod     string
		expectedMethod string
	}{
		{
			name:           "ScramSHA256",
			authMethod:     "scram-sha-256",
			expectedMethod: "scram-sha-256",
		},
		{
			name:           "MD5",
			authMethod:     "md5",
			expectedMethod: "md5",
		},
		{
			name:           "Trust",
			authMethod:     "trust",
			expectedMethod: "trust",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			hbaPath := filepath.Join(tmpDir, "pg_hba.conf")

			initialContent := "# PostgreSQL Client Authentication Configuration File\n"
			if err := os.WriteFile(hbaPath, []byte(initialContent), 0600); err != nil {
				t.Fatal(err)
			}

			p := &Postgres{
				Config:  &pkg.PostgresConf{},
				DataDir: tmpDir,
			}

			err := p.SetupPgHBA(tt.authMethod)
			if err != nil {
				t.Fatalf("SetupPgHBA failed: %v", err)
			}

			content, err := os.ReadFile(hbaPath)
			if err != nil {
				t.Fatal(err)
			}

			contentStr := string(content)

			if !strings.Contains(contentStr, initialContent) {
				t.Error("Original content not preserved")
			}

			expectedLine := "host all all all " + tt.expectedMethod
			if !strings.Contains(contentStr, expectedLine) {
				t.Errorf("Expected line not found: %s\nContent:\n%s", expectedLine, contentStr)
			}
		})
	}
}

func TestSetupPgHBANoDataDir(t *testing.T) {
	p := &Postgres{
		Config: &pkg.PostgresConf{},
	}

	err := p.SetupPgHBA("scram-sha-256")
	if err == nil {
		t.Error("Expected error when DataDir is not set")
	}
}

func TestSetupPgHBANonExistentFile(t *testing.T) {
	p := &Postgres{
		Config:  &pkg.PostgresConf{},
		DataDir: "/nonexistent",
	}

	err := p.SetupPgHBA("scram-sha-256")
	if err == nil {
		t.Error("Expected error for nonexistent pg_hba.conf")
	}
}
