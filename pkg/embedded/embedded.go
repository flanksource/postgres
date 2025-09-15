package embedded

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/flanksource/commons/deps"
	"github.com/flanksource/postgres/pkg/schemas"
)

// MinimalPostgres provides just the functionality needed for schema generation
type MinimalPostgres struct {
	BinDir string
}

// DescribeConfig executes `postgres --describe-config` and returns parsed parameters
func (p *MinimalPostgres) DescribeConfig() ([]schemas.Param, error) {
	if p.BinDir == "" {
		return nil, fmt.Errorf("postgres binary directory not set")
	}

	postgresPath := filepath.Join(p.BinDir, "postgres")

	// Execute postgres --describe-config
	cmd := exec.Command(postgresPath, "--describe-config")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run postgres --describe-config: %w", err)
	}

	return schemas.ParseDescribeConfig(string(output))
}

// EmbeddedPostgres manages an embedded PostgreSQL instance for schema generation
type EmbeddedPostgres struct {
	*MinimalPostgres
	Version string
	TempDir string
}

// NewEmbeddedPostgres creates a new embedded PostgreSQL instance for schema generation
func NewEmbeddedPostgres(version string) (*EmbeddedPostgres, error) {
	if version == "" {
		version = "17.6.0" // Default version
	}

	// Create a temporary directory for the embedded postgres
	tempDir := filepath.Join(os.TempDir(), "embedded-postgres")

	// Download and install postgres using commons/deps
	err := deps.Install("postgres", version, deps.WithBinDir(tempDir))
	if err != nil {
		return nil, fmt.Errorf("failed to install embedded postgres: %w", err)
	}

	// The postgres binaries will be in tempDir/postgres/bin/
	binDir := filepath.Join(tempDir, "postgres", "bin")

	// Verify that the postgres binary exists
	postgresPath := filepath.Join(binDir, "postgres")
	if _, err := os.Stat(postgresPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("postgres binary not found at %s", postgresPath)
	}

	postgres := &MinimalPostgres{
		BinDir: binDir,
	}

	return &EmbeddedPostgres{
		MinimalPostgres: postgres,
		Version:         version,
		TempDir:         tempDir,
	}, nil
}

// Cleanup removes the embedded PostgreSQL installation
func (e *EmbeddedPostgres) Cleanup() error {
	if e.TempDir != "" {
		return os.RemoveAll(e.TempDir)
	}
	return nil
}