package health

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

// MemoryUsageChecker monitors system memory usage
type MemoryUsageChecker struct {
	Threshold float64 // Percentage threshold
}

// NewMemoryUsageChecker creates a new memory usage health checker
func NewMemoryUsageChecker(threshold float64) *MemoryUsageChecker {
	return &MemoryUsageChecker{
		Threshold: threshold,
	}
}

// Status implements the health.ICheckable interface
func (c *MemoryUsageChecker) Status() (interface{}, error) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Get system memory info
	var totalMem uint64
	var availMem uint64

	// On Linux, read /proc/meminfo
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile("/proc/meminfo")
		if err != nil {
			return nil, fmt.Errorf("failed to read memory info: %w", err)
		}

		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) < 3 {
				continue
			}

			var value uint64
			fmt.Sscanf(fields[1], "%d", &value)
			value *= 1024 // Convert kB to bytes

			switch fields[0] {
			case "MemTotal:":
				totalMem = value
			case "MemAvailable:":
				availMem = value
			}
		}
	} else {
		// Fallback for other systems - use Go runtime stats as approximation
		totalMem = m.Sys
		availMem = totalMem - m.Alloc
	}

	if totalMem == 0 {
		return nil, fmt.Errorf("unable to determine system memory")
	}

	usedMem := totalMem - availMem
	usagePercent := float64(usedMem) / float64(totalMem) * 100

	status := map[string]interface{}{
		"total_memory":      totalMem,
		"used_memory":       usedMem,
		"available_memory":  availMem,
		"usage_percent":     usagePercent,
		"threshold_percent": c.Threshold,
		"go_allocated":      m.Alloc,
		"go_sys":            m.Sys,
		"timestamp":         time.Now(),
	}

	if usagePercent > c.Threshold {
		return status, fmt.Errorf("memory usage %.2f%% exceeds threshold %.2f%%", usagePercent, c.Threshold)
	}

	status["status"] = "healthy"
	return status, nil
}
