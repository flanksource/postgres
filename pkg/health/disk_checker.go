package health

import (
	"fmt"
	"syscall"
	"time"
)

// DiskSpaceChecker monitors disk space usage
type DiskSpaceChecker struct {
	Path      string
	Threshold float64 // Percentage threshold (e.g., 90.0 for 90%)
}

// NewDiskSpaceChecker creates a new disk space health checker
func NewDiskSpaceChecker(path string, threshold float64) *DiskSpaceChecker {
	return &DiskSpaceChecker{
		Path:      path,
		Threshold: threshold,
	}
}

// Status implements the health.ICheckable interface
func (c *DiskSpaceChecker) Status() (interface{}, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(c.Path, &stat)
	if err != nil {
		return nil, fmt.Errorf("failed to get disk stats for %s: %w", c.Path, err)
	}

	// Calculate usage percentage
	totalBytes := stat.Blocks * uint64(stat.Bsize)
	availableBytes := stat.Bavail * uint64(stat.Bsize)
	usedBytes := totalBytes - availableBytes
	usagePercent := float64(usedBytes) / float64(totalBytes) * 100

	status := map[string]interface{}{
		"path":              c.Path,
		"total_bytes":       totalBytes,
		"used_bytes":        usedBytes,
		"available_bytes":   availableBytes,
		"usage_percent":     usagePercent,
		"threshold_percent": c.Threshold,
		"timestamp":         time.Now(),
	}

	if usagePercent > c.Threshold {
		return status, fmt.Errorf("disk usage %.2f%% exceeds threshold %.2f%%", usagePercent, c.Threshold)
	}

	status["status"] = "healthy"
	return status, nil
}
