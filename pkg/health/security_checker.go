package health

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// SecurityChecker validates security configuration
type SecurityChecker struct {
	DataDir string
}

// NewSecurityChecker creates a new security health checker
func NewSecurityChecker(dataDir string) *SecurityChecker {
	return &SecurityChecker{
		DataDir: dataDir,
	}
}

// Status implements the health.ICheckable interface
func (c *SecurityChecker) Status() (interface{}, error) {
	issues := []string{}

	// Check data directory permissions
	if c.DataDir != "" {
		info, err := os.Stat(c.DataDir)
		if err != nil {
			return nil, fmt.Errorf("cannot access data directory: %w", err)
		}

		mode := info.Mode()
		if mode.Perm() != 0700 {
			issues = append(issues, fmt.Sprintf("data directory has insecure permissions: %o (should be 0700)", mode.Perm()))
		}

		// Check if files are owned by current user
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			currentUID := uint32(os.Getuid())
			if stat.Uid != currentUID {
				issues = append(issues, "data directory not owned by current user")
			}
		}
	}

	// Check for common insecure files
	insecureFiles := []string{
		filepath.Join(c.DataDir, "postgresql.conf"),
		filepath.Join(c.DataDir, "pg_hba.conf"),
	}

	for _, file := range insecureFiles {
		if info, err := os.Stat(file); err == nil {
			mode := info.Mode()
			if mode.Perm()&0077 != 0 {
				issues = append(issues, fmt.Sprintf("%s has insecure permissions: %o", file, mode.Perm()))
			}
		}
	}

	status := map[string]interface{}{
		"data_dir":  c.DataDir,
		"timestamp": time.Now(),
		"issues":    issues,
	}

	if len(issues) > 0 {
		return status, fmt.Errorf("security issues found: %s", strings.Join(issues, "; "))
	}

	status["status"] = "healthy"
	return status, nil
}
