package health

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// BackupHealthChecker monitors backup status
type BackupHealthChecker struct {
	BackupLocation string
	MaxAge         time.Duration
}

// NewBackupHealthChecker creates a new backup health checker
func NewBackupHealthChecker(backupLocation string, maxAge time.Duration) *BackupHealthChecker {
	return &BackupHealthChecker{
		BackupLocation: backupLocation,
		MaxAge:         maxAge,
	}
}

// Status implements the health.ICheckable interface
func (c *BackupHealthChecker) Status() (interface{}, error) {
	if c.BackupLocation == "" {
		return nil, fmt.Errorf("backup location not configured")
	}

	// Find the most recent backup file
	var latestBackup os.FileInfo
	var latestTime time.Time

	err := filepath.Walk(c.BackupLocation, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		if !info.IsDir() && (strings.HasSuffix(path, ".sql") || strings.HasSuffix(path, ".dump") || strings.HasSuffix(path, ".backup")) {
			if info.ModTime().After(latestTime) {
				latestBackup = info
				latestTime = info.ModTime()
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan backup location: %w", err)
	}

	status := map[string]interface{}{
		"backup_location": c.BackupLocation,
		"max_age":         c.MaxAge,
		"timestamp":       time.Now(),
	}

	if latestBackup == nil {
		return status, fmt.Errorf("no backup files found in %s", c.BackupLocation)
	}

	age := time.Since(latestTime)
	status["latest_backup"] = latestBackup.Name()
	status["latest_backup_time"] = latestTime
	status["backup_age"] = age

	if age > c.MaxAge {
		return status, fmt.Errorf("latest backup is %v old (max allowed: %v)", age, c.MaxAge)
	}

	status["status"] = "healthy"
	return status, nil
}
