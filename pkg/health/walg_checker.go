package health

import (
	"fmt"
	"time"
)

// WalG interface defines the methods needed for WAL-G health checking
type WalG interface {
	Health() error
}

// WalgChecker implements health check for WAL-G backup service
type WalgChecker struct {
	walg WalG
}

// NewWalgChecker creates a new WAL-G health checker
func NewWalgChecker(walg WalG) *WalgChecker {
	return &WalgChecker{
		walg: walg,
	}
}

// Status performs a WAL-G health check
func (w *WalgChecker) Status() (interface{}, error) {
	if w.walg == nil {
		return map[string]interface{}{
			"status":    "unknown",
			"timestamp": time.Now(),
			"service":   "walg",
			"error":     "service not configured",
		}, fmt.Errorf("WAL-G service not configured")
	}
	
	err := w.walg.Health()
	if err != nil {
		return map[string]interface{}{
			"status":    "unhealthy",
			"timestamp": time.Now(),
			"service":   "walg",
		}, err
	}
	
	return map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"service":   "walg",
	}, nil
}