package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/flanksource/clicky"
	"github.com/flanksource/commons/logger"
)

// SetupPgHBA configures pg_hba.conf for host authentication
// Detects the default password encryption method and configures authentication accordingly
func (p *Postgres) SetupPgHBA(authMethod string) error {

	hbaPath := filepath.Join(p.DataDir, "pg_hba.conf")

	if authMethod == "" {
		detected, err := p.detectPasswordEncryption()
		if err != nil {
			logger.Warnf("Failed to detect password encryption method, using 'scram-sha-256': %v", err)
			authMethod = "scram-sha-256"
		} else {
			authMethod = detected
		}
	}

	if authMethod == "trust" {
		logger.Warnf("WARNING: Using 'trust' authentication allows anyone to connect without a password!")
		logger.Warnf("This is NOT recommended for production use!")
	}

	rule := fmt.Sprintf("\n# Added by postgres auto-configuration\nhost all all all %s\n", authMethod)

	f, err := os.OpenFile(hbaPath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open pg_hba.conf: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(rule); err != nil {
		return fmt.Errorf("failed to write to pg_hba.conf: %w", err)
	}

	logger.Infof("Configured pg_hba.conf with authentication method: %s", authMethod)
	return nil
}

// detectPasswordEncryption queries postgres binary for the default password encryption method
func (p *Postgres) detectPasswordEncryption() (string, error) {
	if p.BinDir == "" {
		return "", fmt.Errorf("postgres binary directory not set")
	}

	postgresPath := filepath.Join(p.BinDir, "postgres")

	process := clicky.Exec(postgresPath, "-C", "password_encryption").Run()
	if process.Err != nil {
		return "", fmt.Errorf("failed to detect password_encryption: %w", process.Err)
	}

	method := strings.TrimSpace(process.GetStdout())

	switch method {
	case "md5":
		return "md5", nil
	case "scram-sha-256":
		return "scram-sha-256", nil
	default:
		return "scram-sha-256", nil
	}
}
