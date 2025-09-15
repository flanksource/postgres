package health

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/invisionapp/go-health"

	"github.com/flanksource/postgres/pkg/jwt"
)

// HealthChecker provides comprehensive health checks for PostgreSQL and related services
type HealthChecker struct {
	h      *health.Health
	config *Config
}

// Config contains configuration for health checks
type Config struct {
	// Service instances
	PostgresService  Postgres
	PgBouncerService PgBouncer
	WalgService      WalG

	// PostgREST configuration
	PostgRESTURL   string
	JWTGenerator   *jwt.JWTGenerator
	PostgRESTAdmin string

	// File system paths to monitor
	DataDir        string
	WALDir         string
	BackupLocation string

	// Supervisor configuration
	SupervisorEnabled bool     // Whether to monitor supervisor
	EnabledServices   []string // Services that should be running

	// Thresholds
	DiskSpaceThreshold   float64 // Percentage (e.g., 90.0 for 90%)
	WALSizeThreshold     int64   // Bytes
	MemoryUsageThreshold float64 // Percentage
	CPUUsageThreshold    float64 // Percentage
}

// NewHealthChecker creates a new health checker with comprehensive checks
func NewHealthChecker(config *Config) (*HealthChecker, error) {
	if config == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}

	h := health.New()
	h.DisableLogging() // We'll handle logging ourselves

	checker := &HealthChecker{
		h:      h,
		config: config,
	}

	if err := checker.setupChecks(); err != nil {
		return nil, fmt.Errorf("failed to setup health checks: %w", err)
	}

	return checker, nil
}

// setupChecks configures all health checks
func (hc *HealthChecker) setupChecks() error {
	if hc == nil || hc.config == nil {
		return fmt.Errorf("health checker or configuration is nil")
	}

	// PostgreSQL database connectivity check
	if hc.config.PostgresService != nil {
		if err := hc.addPostgreSQLCheck(hc.config.PostgresService); err != nil {
			return fmt.Errorf("failed to add PostgreSQL check: %w", err)
		}
	}

	// PostgREST API health check with JWT authentication
	if hc.config.PostgRESTURL != "" {
		if err := hc.addPostgRESTCheck(); err != nil {
			return fmt.Errorf("failed to add PostgREST check: %w", err)
		}
	}

	// PgBouncer connection pooler check
	if hc.config.PgBouncerService != nil {
		if err := hc.addPgBouncerCheck(hc.config.PgBouncerService); err != nil {
			return fmt.Errorf("failed to add PgBouncer check: %w", err)
		}
	}

	// Disk space checks
	if hc.config.DataDir != "" {
		if err := hc.addDiskSpaceCheck(); err != nil {
			return fmt.Errorf("failed to add disk space check: %w", err)
		}
	}

	// WAL size check
	if hc.config.WALDir != "" {
		if err := hc.addWALSizeCheck(); err != nil {
			return fmt.Errorf("failed to add WAL size check: %w", err)
		}
	}

	// System resource checks
	if err := hc.addSystemResourceChecks(); err != nil {
		return fmt.Errorf("failed to add system resource checks: %w", err)
	}

	// Security and configuration checks
	if err := hc.addSecurityChecks(); err != nil {
		return fmt.Errorf("failed to add security checks: %w", err)
	}

	// Backup health check
	if hc.config.BackupLocation != "" {
		if err := hc.addBackupHealthCheck(); err != nil {
			return fmt.Errorf("failed to add backup health check: %w", err)
		}
	}

	// WAL-G backup service check
	if hc.config.WalgService != nil {
		if err := hc.addWalgCheck(); err != nil {
			return fmt.Errorf("failed to add WAL-G check: %w", err)
		}
	}

	// Supervisor health check
	if hc.config.SupervisorEnabled {
		if err := hc.addSupervisorCheck(); err != nil {
			return fmt.Errorf("failed to add supervisor check: %w", err)
		}
	}

	return nil
}

// addPostgreSQLCheck adds PostgreSQL database health check using Postgres.SQL method
func (hc *HealthChecker) addPostgreSQLCheck(postgres Postgres) error {
	checker := NewPostgreSQLChecker(postgres)

	return hc.h.AddCheck(&health.Config{
		Name:     "postgres-db",
		Checker:  checker,
		Interval: 30 * time.Second,
		Fatal:    true,
	})
}

// addPostgRESTCheck adds PostgREST API health check with JWT authentication
func (hc *HealthChecker) addPostgRESTCheck() error {
	checker := NewPostgRESTChecker(
		hc.config.PostgRESTURL,
		hc.config.JWTGenerator,
		hc.config.PostgRESTAdmin,
		10*time.Second,
	)

	return hc.h.AddCheck(&health.Config{
		Name:     "postgrest-api",
		Checker:  checker,
		Interval: 60 * time.Second,
		Fatal:    false,
	})
}

// addPgBouncerCheck adds PgBouncer connection pooler check
func (hc *HealthChecker) addPgBouncerCheck(pgbouncer PgBouncer) error {
	checker := NewPgBouncerChecker(pgbouncer)

	return hc.h.AddCheck(&health.Config{
		Name:     "pgbouncer",
		Checker:  checker,
		Interval: 30 * time.Second,
		Fatal:    false,
	})
}

// addDiskSpaceCheck adds disk space monitoring
func (hc *HealthChecker) addDiskSpaceCheck() error {
	checker := NewDiskSpaceChecker(hc.config.DataDir, hc.config.DiskSpaceThreshold)

	return hc.h.AddCheck(&health.Config{
		Name:     "disk-space",
		Checker:  checker,
		Interval: 60 * time.Second,
		Fatal:    false,
	})
}

// addWALSizeCheck adds WAL directory size monitoring
func (hc *HealthChecker) addWALSizeCheck() error {
	checker := NewWALSizeChecker(hc.config.WALDir, hc.config.WALSizeThreshold)

	return hc.h.AddCheck(&health.Config{
		Name:     "wal-size",
		Checker:  checker,
		Interval: 300 * time.Second, // Check every 5 minutes
		Fatal:    false,
	})
}

// addSystemResourceChecks adds memory and CPU usage monitoring
func (hc *HealthChecker) addSystemResourceChecks() error {
	// Memory usage check
	memChecker := NewMemoryUsageChecker(hc.config.MemoryUsageThreshold)

	if err := hc.h.AddCheck(&health.Config{
		Name:     "memory-usage",
		Checker:  memChecker,
		Interval: 30 * time.Second,
		Fatal:    false,
	}); err != nil {
		return err
	}

	// CPU usage check
	cpuChecker := NewCPUUsageChecker(hc.config.CPUUsageThreshold)

	return hc.h.AddCheck(&health.Config{
		Name:     "cpu-usage",
		Checker:  cpuChecker,
		Interval: 30 * time.Second,
		Fatal:    false,
	})
}

// addSecurityChecks adds security configuration validation
func (hc *HealthChecker) addSecurityChecks() error {
	checker := NewSecurityChecker(hc.config.DataDir)

	return hc.h.AddCheck(&health.Config{
		Name:     "security-config",
		Checker:  checker,
		Interval: 300 * time.Second, // Check every 5 minutes
		Fatal:    false,
	})
}

// addBackupHealthCheck adds backup status monitoring
func (hc *HealthChecker) addBackupHealthCheck() error {
	checker := NewBackupHealthChecker(hc.config.BackupLocation, 24*time.Hour)

	return hc.h.AddCheck(&health.Config{
		Name:     "backup-status",
		Checker:  checker,
		Interval: 600 * time.Second, // Check every 10 minutes
		Fatal:    false,
	})
}

// addWalgCheck adds WAL-G service health check
func (hc *HealthChecker) addWalgCheck() error {
	checker := NewWalgChecker(hc.config.WalgService)

	return hc.h.AddCheck(&health.Config{
		Name:     "walg-service",
		Checker:  checker,
		Interval: 300 * time.Second, // Check every 5 minutes
		Fatal:    false,
	})
}

// addSupervisorCheck adds supervisor process monitoring
func (hc *HealthChecker) addSupervisorCheck() error {
	checker := NewSupervisorChecker(hc.config.EnabledServices)

	return hc.h.AddCheck(&health.Config{
		Name:     "supervisor",
		Checker:  checker,
		Interval: 60 * time.Second, // Check every minute
		Fatal:    false,            // Supervisor issues shouldn't be fatal
	})
}

// Start begins health monitoring
func (hc *HealthChecker) Start() error {
	return hc.h.Start()
}

// Stop stops health monitoring
func (hc *HealthChecker) Stop() error {
	return hc.h.Stop()
}

// GetStatus returns current health status
func (hc *HealthChecker) GetStatus() map[string]interface{} {
	status, _, _ := hc.h.State()
	result := make(map[string]interface{})
	for k, v := range status {
		result[k] = v
	}
	return result
}

// IsHealthy returns true if all fatal checks are passing
func (hc *HealthChecker) IsHealthy() bool {
	_, failed, err := hc.h.State()
	return err == nil && !failed
}

// Handler returns an HTTP handler for health checks (simple status)
func (hc *HealthChecker) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, failed, err := hc.h.State()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if failed {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status": "unhealthy"}`))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "healthy"}`))
		}
	}
}

// StatusHandler returns an HTTP handler that provides detailed health status
// This returns the complete go-health state as per the user's requirement
func (hc *HealthChecker) StatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, failed, err := hc.h.State()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if failed {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		// Convert go-health state to JSON response
		response := map[string]interface{}{
			"status": "healthy",
			"checks": make(map[string]interface{}),
		}

		if failed {
			response["status"] = "unhealthy"
		}

		// Add detailed check information
		for name, state := range status {
			checkInfo := map[string]interface{}{
				"status":     state.Status,
				"last_check": state.CheckTime,
			}

			if state.Details != nil {
				checkInfo["details"] = state.Details
			}

			if state.Err != "" {
				checkInfo["error"] = state.Err
			}

			response["checks"].(map[string]interface{})[name] = checkInfo
		}

		json.NewEncoder(w).Encode(response)
	}
}
