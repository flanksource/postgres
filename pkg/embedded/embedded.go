package embedded

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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
// Note: This currently requires PostgreSQL to be installed on the system
func NewEmbeddedPostgres(version string) (*EmbeddedPostgres, error) {
	if version == "" {
		version = "17" // Default version
	}

	// For now, we'll look for system-installed PostgreSQL
	// In the future, this could use flanksource/deps to download PostgreSQL binaries
	binDir := fmt.Sprintf("/usr/lib/postgresql/%s/bin", version)

	// Check if the postgres binary exists
	postgresPath := filepath.Join(binDir, "postgres")
	if _, err := os.Stat(postgresPath); os.IsNotExist(err) {
		// Try alternative locations
		alternativePaths := []string{
			"/usr/local/pgsql/bin",
			"/usr/pgsql-" + version + "/bin",
			"/opt/postgresql/" + version + "/bin",
		}

		found := false
		for _, altPath := range alternativePaths {
			altPostgresPath := filepath.Join(altPath, "postgres")
			if _, err := os.Stat(altPostgresPath); err == nil {
				binDir = altPath
				found = true
				break
			}
		}

		if !found {
			return nil, fmt.Errorf("postgres binary not found - please install PostgreSQL %s", version)
		}
	}

	postgres := &MinimalPostgres{
		BinDir: binDir,
	}

	return &EmbeddedPostgres{
		MinimalPostgres: postgres,
		Version:         version,
		TempDir:         "", // No temp dir needed for system installation
	}, nil
}

// Cleanup removes the embedded PostgreSQL installation
func (e *EmbeddedPostgres) Cleanup() error {
	// Since we're using system PostgreSQL, there's nothing to clean up
	if e.TempDir != "" {
		return os.RemoveAll(e.TempDir)
	}
	return nil
}
