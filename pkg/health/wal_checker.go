package health

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// WALSizeChecker monitors WAL directory size
type WALSizeChecker struct {
	WALDir    string
	Threshold int64 // Size threshold in bytes
}

// NewWALSizeChecker creates a new WAL size health checker
func NewWALSizeChecker(walDir string, threshold int64) *WALSizeChecker {
	return &WALSizeChecker{
		WALDir:    walDir,
		Threshold: threshold,
	}
}

// Status implements the health.ICheckable interface
func (c *WALSizeChecker) Status() (interface{}, error) {
	var totalSize int64

	err := filepath.Walk(c.WALDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to calculate WAL directory size: %w", err)
	}

	status := map[string]interface{}{
		"wal_dir":        c.WALDir,
		"total_size":     totalSize,
		"threshold_size": c.Threshold,
		"timestamp":      time.Now(),
	}

	if totalSize > c.Threshold {
		return status, fmt.Errorf("WAL size %d bytes exceeds threshold %d bytes", totalSize, c.Threshold)
	}

	status["status"] = "healthy"
	return status, nil
}
