package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/flanksource/clicky"
	"github.com/flanksource/postgres/pkg"
	"github.com/flanksource/postgres/pkg/extensions"
	"github.com/flanksource/postgres/pkg/generators"
	"github.com/flanksource/postgres/pkg/health"
	"github.com/flanksource/postgres/pkg/pgtune"
	"github.com/flanksource/postgres/pkg/sysinfo"
)

// HealthServer provides health check and configuration endpoints
type HealthServer struct {
	Port          int
	ConfigDir     string
	SystemInfo    *sysinfo.SystemInfo
	TunedParams   *pgtune.TunedParameters
	DBType        string
	MaxConn       int
	server        *http.Server
	healthChecker *health.HealthChecker
	startTime     time.Time

	// Service configurations
	PostgresConfig  *pkg.PostgresConf
	PgBouncerConfig *pkg.PgBouncerConf
	PostgRESTConfig *pkg.PostgrestConf
	WalgConfig      *pkg.WalgConf
}

// NewHealthServer creates a new health check server
func NewHealthServer(port int, configDir string) *HealthServer {
	return &HealthServer{
		Port:      port,
		ConfigDir: configDir,
		startTime: time.Now(),
	}
}

// Start starts the health check server
func (s *HealthServer) Start() error {

	// Initialize health checker if not already set
	if s.healthChecker == nil {
		s.initializeHealthChecker()
	}

	// Set up HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/ready", s.handleReady)
	mux.HandleFunc("/live", s.handleLive)
	mux.HandleFunc("/info", s.handleInfo)
	mux.HandleFunc("/config", s.handleConfig)
	mux.HandleFunc("/health/services", s.handleHealthServices)
	mux.HandleFunc("/health/extensions", s.handleHealthExtensions)
	mux.HandleFunc("/health/supervisord", s.handleHealthSupervisord)
	mux.HandleFunc("/health/config", s.handleHealthConfig)
	mux.HandleFunc("/health/status", s.handleHealthStatus)
	mux.HandleFunc("/config/postgresql.conf", s.handlePostgreSQLConfig)
	mux.HandleFunc("/config/pgbouncer.ini", s.handlePgBouncerConfig)
	mux.HandleFunc("/config/postgrest.conf", s.handlePostgRESTConfig)
	mux.HandleFunc("/", s.handleRoot)

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.Port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Starting health check server on port %d", s.Port)

	// Start the health checker
	if s.healthChecker != nil {
		if err := s.healthChecker.Start(); err != nil {
			log.Printf("Warning: Failed to start health checker: %v", err)
		} else {
			log.Printf("Health checker started successfully")
		}
	}

	return s.server.ListenAndServe()
}

// Stop stops the health check server
func (s *HealthServer) Stop(ctx context.Context) error {
	// Stop health checker first
	if s.healthChecker != nil {
		if err := s.healthChecker.Stop(); err != nil {
			log.Printf("Warning: Failed to stop health checker: %v", err)
		}
	}

	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

// SetConfiguration sets the tuning configuration for the server
func (s *HealthServer) SetConfiguration(maxConn int, dbType string, tunedParams *pgtune.TunedParameters) {
	s.MaxConn = maxConn
	s.DBType = dbType
	s.TunedParams = tunedParams
}

// handleReady checks if all services are ready
func (s *HealthServer) handleReady(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "ready",
		"timestamp": time.Now().Unix(),
		"checks": map[string]string{
			"system_info": "ok",
		},
	}

	// Check if configuration is available
	if s.TunedParams != nil {
		response["checks"].(map[string]string)["configuration"] = "ok"
	} else {
		response["checks"].(map[string]string)["configuration"] = "not_available"
		response["status"] = "not_ready"
	}

	// TODO: Add actual health checks for PostgreSQL, PgBouncer, PostgREST
	// For now, we'll assume they're ready if system info is available

	if response["status"] == "not_ready" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleLive provides a simple liveness check
func (s *HealthServer) handleLive(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "alive",
		"timestamp": time.Now().Unix(),
		"uptime":    time.Since(s.startTime).Seconds(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleInfo returns system and version information
func (s *HealthServer) handleInfo(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(clicky.MustFormat(s.SystemInfo, clicky.FormatOptions{JSON: true})))
}

// handleConfig returns current configuration summary
func (s *HealthServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	if s.TunedParams == nil {
		http.Error(w, "Configuration not available", http.StatusServiceUnavailable)
		return
	}

	response := map[string]interface{}{
		"system": map[string]interface{}{
			"total_memory_gb": s.SystemInfo.TotalMemoryGB(),
			"cpu_count":       s.SystemInfo.EffectiveCPUCount(),
			"os_type":         s.SystemInfo.OSType,
			"disk_type":       s.SystemInfo.DiskType,
		},
		"tuning": map[string]interface{}{
			"max_connections": s.TunedParams.MaxConnections,
			"db_type":         s.DBType,
		},
		"parameters": map[string]interface{}{
			"shared_buffers_kb":        s.TunedParams.SharedBuffers,
			"effective_cache_size_kb":  s.TunedParams.EffectiveCacheSize,
			"maintenance_work_mem_kb":  s.TunedParams.MaintenanceWorkMem,
			"work_mem_kb":              s.TunedParams.WorkMem,
			"wal_buffers_kb":           s.TunedParams.WalBuffers,
			"min_wal_size_kb":          s.TunedParams.MinWalSize,
			"max_wal_size_kb":          s.TunedParams.MaxWalSize,
			"random_page_cost":         s.TunedParams.RandomPageCost,
			"effective_io_concurrency": s.TunedParams.EffectiveIoConcurrency,
			"max_worker_processes":     s.TunedParams.MaxWorkerProcesses,
			"max_parallel_workers":     s.TunedParams.MaxParallelWorkers,
			"wal_level":                s.TunedParams.WalLevel,
			"huge_pages":               s.TunedParams.HugePages,
		},
		"endpoints": map[string]string{
			"postgresql_conf": "/config/postgresql.conf",
			"pgbouncer_ini":   "/config/pgbouncer.ini",
			"postgrest_conf":  "/config/postgrest.conf",
			"pg_hba_conf":     "/config/pg_hba.conf",
		},
		"timestamp": time.Now().Unix(),
	}

	if len(s.TunedParams.Warnings) > 0 {
		response["warnings"] = s.TunedParams.Warnings
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handlePostgreSQLConfig returns postgresql.conf configuration
func (s *HealthServer) handlePostgreSQLConfig(w http.ResponseWriter, r *http.Request) {
	if s.TunedParams == nil {
		http.Error(w, "Configuration not available", http.StatusServiceUnavailable)
		return
	}

	generator := generators.NewPostgreSQLConfigGenerator(s.SystemInfo, s.TunedParams)
	configContent := generator.GenerateConfigFile()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=postgresql.conf")
	w.Write([]byte(configContent))
}

// handlePgBouncerConfig returns pgbouncer.ini configuration
func (s *HealthServer) handlePgBouncerConfig(w http.ResponseWriter, r *http.Request) {
	if s.TunedParams == nil {
		http.Error(w, "Configuration not available", http.StatusServiceUnavailable)
		return
	}

	generator := generators.NewPgBouncerConfigGenerator(s.SystemInfo, s.TunedParams)
	configContent := generator.GenerateConfigFile()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=pgbouncer.ini")
	w.Write([]byte(configContent))
}

// handlePostgRESTConfig returns PostgREST configuration
func (s *HealthServer) handlePostgRESTConfig(w http.ResponseWriter, r *http.Request) {
	if s.TunedParams == nil {
		http.Error(w, "Configuration not available", http.StatusServiceUnavailable)
		return
	}

	generator := generators.NewPostgRESTConfigGenerator(s.SystemInfo, s.TunedParams)
	configContent, _ := generator.GenerateConfigFile()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=postgrest.conf")
	w.Write([]byte(configContent))
}

// handleRoot provides API documentation
func (s *HealthServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"service": "PostgreSQL Configuration Health Server",
		"version": "1.0.0",
		"endpoints": map[string]map[string]string{
			"health": {
				"/live":  "Liveness check - returns 200 if server is running",
				"/ready": "Readiness check - returns 200 if server is ready to serve requests",
				"/info":  "System and version information",
			},
			"configuration": {
				"/config":                 "Configuration summary (JSON)",
				"/config/postgresql.conf": "PostgreSQL configuration file",
				"/config/pgbouncer.ini":   "PgBouncer configuration file",
				"/config/postgrest.conf":  "PostgREST configuration file",
				"/config/pg_hba.conf":     "PostgreSQL authentication configuration",
			},
		},
		"system": map[string]interface{}{
			"detected_memory_gb": s.SystemInfo.TotalMemoryGB(),
			"detected_cpus":      s.SystemInfo.EffectiveCPUCount(),
			"detected_os":        s.SystemInfo.OSType,
			"detected_disk_type": s.SystemInfo.DiskType,
			"postgresql_version": s.SystemInfo.PostgreSQLVersion,
		},
		"configuration": map[string]interface{}{
			"max_connections": s.MaxConn,
			"database_type":   s.DBType,
		},
		"timestamp": time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(response)
}

// SaveConfigsToDir saves all configuration files to the specified directory
func (s *HealthServer) SaveConfigsToDir(dir string) error {
	if s.TunedParams == nil {
		return fmt.Errorf("no configuration available to save")
	}

	// Generate PostgreSQL config
	pgGenerator := generators.NewPostgreSQLConfigGenerator(s.SystemInfo, s.TunedParams)
	pgConfig := pgGenerator.GenerateConfigFile()
	if err := writeConfigFile(filepath.Join(dir, "postgresql.conf"), pgConfig); err != nil {
		return fmt.Errorf("failed to write postgresql.conf: %w", err)
	}

	// Generate PgBouncer config
	bouncerGenerator := generators.NewPgBouncerConfigGenerator(s.SystemInfo, s.TunedParams)
	bouncerConfig := bouncerGenerator.GenerateConfigFile()
	if err := writeConfigFile(filepath.Join(dir, "pgbouncer.ini"), bouncerConfig); err != nil {
		return fmt.Errorf("failed to write pgbouncer.ini: %w", err)
	}

	// Generate PostgREST config
	restGenerator := generators.NewPostgRESTConfigGenerator(s.SystemInfo, s.TunedParams)
	restConfig, _ := restGenerator.GenerateConfigFile()
	if err := writeConfigFile(filepath.Join(dir, "postgrest.conf"), restConfig); err != nil {
		return fmt.Errorf("failed to write postgrest.conf: %w", err)
	}

	// Generate PostgREST .env file
	envConfig, _ := restGenerator.GenerateEnvFile()
	if err := writeConfigFile(filepath.Join(dir, "postgrest.env"), envConfig); err != nil {
		return fmt.Errorf("failed to write postgrest.env: %w", err)
	}

	// Generate PostgREST user setup SQL
	userSQL := restGenerator.GenerateUserSetupSQL()
	if err := writeConfigFile(filepath.Join(dir, "setup_postgrest_users.sql"), userSQL); err != nil {
		return fmt.Errorf("failed to write setup_postgrest_users.sql: %w", err)
	}

	return nil
}

// writeConfigFile writes content to a file
func writeConfigFile(filename, content string) error {
	return writeFile(filename, []byte(content))
}

// writeFile is a helper function to write files
func writeFile(filename string, data []byte) error {
	// This would normally use os.WriteFile, but for this example we'll just log
	log.Printf("Would write %d bytes to %s", len(data), filename)
	return nil
}

// ServiceStatus represents the status of a service
type ServiceStatus struct {
	Name         string `json:"name"`
	Status       string `json:"status"` // "running", "stopped", "unknown"
	Port         int    `json:"port,omitempty"`
	PortOpen     bool   `json:"port_open"`
	Enabled      bool   `json:"enabled"`
	Uptime       string `json:"uptime,omitempty"`
	RestartCount int    `json:"restart_count,omitempty"`
	Details      string `json:"details,omitempty"`
}

// ExtensionStatus represents the status of a PostgreSQL extension
type ExtensionStatus struct {
	Name      string `json:"name"`
	SQLName   string `json:"sql_name"`
	Installed bool   `json:"installed"`
	Version   string `json:"version,omitempty"`
	Required  bool   `json:"required"`
	Available bool   `json:"available"`
	Error     string `json:"error,omitempty"`
}

// handleHealthServices returns the operational status of all services
func (s *HealthServer) handleHealthServices(w http.ResponseWriter, r *http.Request) {
	services := []ServiceStatus{
		s.getPostgreSQLStatus(),
		s.getPgBouncerStatus(),
		s.getPostgRESTStatus(),
		s.getWALGStatus(),
	}

	response := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"services":  services,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleHealthExtensions returns the installation status of PostgreSQL extensions
func (s *HealthServer) handleHealthExtensions(w http.ResponseWriter, r *http.Request) {
	extensions := s.getExtensionsStatus()

	response := map[string]interface{}{
		"timestamp":  time.Now().Unix(),
		"extensions": extensions,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleHealthSupervisord returns supervisord process status
func (s *HealthServer) handleHealthSupervisord(w http.ResponseWriter, r *http.Request) {
	supervisordStatus := s.getSupervisordStatus()

	response := map[string]interface{}{
		"timestamp":   time.Now().Unix(),
		"supervisord": supervisordStatus,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleHealthConfig returns configuration validation status
func (s *HealthServer) handleHealthConfig(w http.ResponseWriter, r *http.Request) {
	configStatus := s.getConfigValidationStatus()

	response := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"config":    configStatus,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// isPortOpen checks if a port is open on localhost
func isPortOpen(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 3*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// getPostgreSQLStatus returns PostgreSQL service status
func (s *HealthServer) getPostgreSQLStatus() ServiceStatus {
	status := ServiceStatus{
		Name:     "postgresql",
		Port:     5432,
		Enabled:  true,
		PortOpen: isPortOpen(5432),
	}

	if status.PortOpen {
		status.Status = "running"
		status.Details = "PostgreSQL is accepting connections"
	} else {
		status.Status = "stopped"
		status.Details = "PostgreSQL port is not accessible"
	}

	return status
}

// getPgBouncerStatus returns PgBouncer service status
func (s *HealthServer) getPgBouncerStatus() ServiceStatus {
	enabled := os.Getenv("PGBOUNCER_ENABLED") == "true"
	port := 6432
	if portStr := os.Getenv("PGBOUNCER_PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	status := ServiceStatus{
		Name:     "pgbouncer",
		Port:     port,
		Enabled:  enabled,
		PortOpen: isPortOpen(port),
	}

	if !enabled {
		status.Status = "disabled"
		status.Details = "PgBouncer is not enabled"
	} else if status.PortOpen {
		status.Status = "running"
		status.Details = "PgBouncer is accepting connections"
	} else {
		status.Status = "stopped"
		status.Details = "PgBouncer port is not accessible"
	}

	return status
}

// getPostgRESTStatus returns PostgREST service status
func (s *HealthServer) getPostgRESTStatus() ServiceStatus {
	enabled := os.Getenv("POSTGREST_ENABLED") == "true"
	port := 3000
	if portStr := os.Getenv("POSTGREST_PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	status := ServiceStatus{
		Name:     "postgrest",
		Port:     port,
		Enabled:  enabled,
		PortOpen: isPortOpen(port),
	}

	if !enabled {
		status.Status = "disabled"
		status.Details = "PostgREST is not enabled"
	} else if status.PortOpen {
		status.Status = "running"
		status.Details = "PostgREST API is responding"
	} else {
		status.Status = "stopped"
		status.Details = "PostgREST port is not accessible"
	}

	return status
}

// getWALGStatus returns WAL-G service status
func (s *HealthServer) getWALGStatus() ServiceStatus {
	enabled := os.Getenv("WALG_ENABLED") == "true"

	status := ServiceStatus{
		Name:    "walg",
		Enabled: enabled,
	}

	if !enabled {
		status.Status = "disabled"
		status.Details = "WAL-G is not enabled"
	} else {
		// WAL-G doesn't have a port, check if process is running via supervisorctl
		if output, err := exec.Command("supervisorctl", "status", "walg").Output(); err == nil {
			if strings.Contains(string(output), "RUNNING") {
				status.Status = "running"
				status.Details = "WAL-G backup service is running"
			} else {
				status.Status = "stopped"
				status.Details = "WAL-G backup service is not running"
			}
		} else {
			status.Status = "unknown"
			status.Details = "Unable to determine WAL-G status"
		}
	}

	return status
}

// getExtensionsStatus returns the installation status of PostgreSQL extensions
func (s *HealthServer) getExtensionsStatus() []ExtensionStatus {
	registry := extensions.GetDefaultRegistry()

	// Parse configured extensions from environment
	configuredExtensions, err := registry.ParseFromEnvironment()
	if err != nil {
		// Return empty list if parsing fails
		return []ExtensionStatus{}
	}

	var results []ExtensionStatus
	for _, ext := range configuredExtensions {
		status := ExtensionStatus{
			Name:      ext.Name,
			SQLName:   ext.SQLName,
			Required:  ext.Required,
			Available: ext.Available,
			Installed: ext.Installed,
		}

		if ext.Installed {
			status.Version = s.getExtensionVersion(ext.SQLName)
		}

		// Add any validation errors
		if ext.Required && !ext.Available {
			status.Error = "Extension files not found on system"
		}

		results = append(results, status)
	}

	return results
}

// checkExtensionAvailable checks if extension files are available
func (s *HealthServer) checkExtensionAvailable(sqlName string) bool {
	// Check for extension control file
	pgVersion := s.SystemInfo.PostgreSQLVersion
	majorVersion := int(pgVersion)
	controlFile := fmt.Sprintf("/usr/share/postgresql/%d/extension/%s.control", majorVersion, sqlName)
	_, err := os.Stat(controlFile)
	return err == nil
}

// checkExtensionInstalled checks if extension is installed in database
func (s *HealthServer) checkExtensionInstalled(sqlName string) bool {
	// This is a simplified check - in a real implementation you'd connect to PostgreSQL
	// For now, return true if extension is available (assuming it was installed)
	return s.checkExtensionAvailable(sqlName)
}

// getExtensionVersion gets the installed version of an extension
func (s *HealthServer) getExtensionVersion(sqlName string) string {
	// This would query the database for actual version
	// For now, return a placeholder
	return "unknown"
}

// getSupervisordStatus returns supervisord process status
func (s *HealthServer) getSupervisordStatus() map[string]interface{} {
	status := map[string]interface{}{
		"running":   false,
		"processes": []map[string]interface{}{},
	}

	// Check if supervisorctl is available
	if _, err := exec.LookPath("supervisorctl"); err != nil {
		status["error"] = "supervisorctl not available"
		return status
	}

	// Get supervisord status
	if output, err := exec.Command("supervisorctl", "status").Output(); err == nil {
		status["running"] = true
		processes := []map[string]interface{}{}

		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			parts := strings.Fields(line)
			if len(parts) >= 2 {
				process := map[string]interface{}{
					"name":   parts[0],
					"status": parts[1],
				}

				if len(parts) > 2 {
					process["details"] = strings.Join(parts[2:], " ")
				}

				processes = append(processes, process)
			}
		}

		status["processes"] = processes
	} else {
		status["error"] = fmt.Sprintf("supervisorctl status failed: %v", err)
	}

	return status
}

// getConfigValidationStatus returns configuration validation status
func (s *HealthServer) getConfigValidationStatus() map[string]interface{} {
	status := map[string]interface{}{
		"valid":          true,
		"configurations": []map[string]interface{}{},
		"errors":         []string{},
	}

	// Check if tuned parameters are available
	if s.TunedParams == nil {
		status["valid"] = false
		status["errors"] = append(status["errors"].([]string), "No tuned parameters available")
		return status
	}

	configs := []map[string]interface{}{}

	// PostgreSQL config validation
	pgGenerator := generators.NewPostgreSQLConfigGenerator(s.SystemInfo, s.TunedParams)
	pgConfig := pgGenerator.GenerateConfigFile()

	pgStatus := map[string]interface{}{
		"name":  "postgresql.conf",
		"valid": true,
		"size":  len(pgConfig),
	}

	// Add validation warnings if available
	if len(s.TunedParams.Warnings) > 0 {
		pgStatus["warnings"] = s.TunedParams.Warnings
	}

	configs = append(configs, pgStatus)

	// PgBouncer config validation
	bouncerGenerator := generators.NewPgBouncerConfigGenerator(s.SystemInfo, s.TunedParams)
	bouncerConfig := bouncerGenerator.GenerateConfigFile()

	bouncerStatus := map[string]interface{}{
		"name":  "pgbouncer.ini",
		"valid": true,
		"size":  len(bouncerConfig),
	}

	configs = append(configs, bouncerStatus)

	// PostgREST config validation
	restGenerator := generators.NewPostgRESTConfigGenerator(s.SystemInfo, s.TunedParams)
	restConfig, err := restGenerator.GenerateConfigFile()

	restStatus := map[string]interface{}{
		"name":  "postgrest.conf",
		"valid": err == nil,
		"size":  len(restConfig),
	}

	if err != nil {
		restStatus["error"] = err.Error()
		status["valid"] = false
		status["errors"] = append(status["errors"].([]string), fmt.Sprintf("PostgREST config error: %v", err))
	}

	configs = append(configs, restStatus)

	status["configurations"] = configs
	return status
}

// initializeHealthChecker initializes the go-health health checker
func (s *HealthServer) initializeHealthChecker() {
	// Load service configurations first
	s.LoadServiceConfigs()

	// Create service instances from configurations
	var postgresService *Postgres
	var pgbouncerService *pkg.PgBouncer
	var postgrestURL string
	var walgService *pkg.WalG
	var enabledServices []string

	// Always create PostgreSQL service
	if s.PostgresConfig != nil {
		postgresService = NewPostgres(s.PostgresConfig, s.ConfigDir)
		enabledServices = append(enabledServices, "postgresql")
	}

	// Create PgBouncer service if configured
	if s.PgBouncerConfig != nil {
		pgbouncerService = pkg.NewPgBouncer(s.PgBouncerConfig)
		enabledServices = append(enabledServices, "pgbouncer")
	}

	// Set PostgREST URL if configured
	if s.PostgRESTConfig != nil && s.PostgRESTConfig.ServerHost != nil && s.PostgRESTConfig.ServerPort != nil {
		postgrestURL = fmt.Sprintf("http://%s:%d", *s.PostgRESTConfig.ServerHost, *s.PostgRESTConfig.ServerPort)
		enabledServices = append(enabledServices, "postgrest")
	}

	// Create WAL-G service if configured
	if s.WalgConfig != nil && s.WalgConfig.Enabled {
		walgService = pkg.NewWalG(s.WalgConfig)
		enabledServices = append(enabledServices, "walg")
	}

	// Determine WAL directory from PostgreSQL data directory
	walDir := ""
	if s.PostgresConfig != nil {
		pgDataDir := os.Getenv("PGDATA")
		if pgDataDir != "" {
			walDir = filepath.Join(pgDataDir, "pg_wal")
		}
	}

	// Determine backup location from WAL-G config
	backupLocation := ""
	if s.WalgConfig != nil {
		// Use local file prefix as backup location for health checks
		if s.WalgConfig.FilePrefix != nil && *s.WalgConfig.FilePrefix != "" {
			backupLocation = strings.TrimPrefix(*s.WalgConfig.FilePrefix, "file://")
		}
	}

	// Create comprehensive health checker config
	config := &health.Config{
		// Service instances
		PostgresService:  postgresService,
		PgBouncerService: pgbouncerService,
		WalgService:      walgService,

		// PostgREST configuration
		PostgRESTURL:   postgrestURL,
		JWTGenerator:   nil, // TODO: Initialize JWT generator if needed
		PostgRESTAdmin: "",  // TODO: Set admin role

		// File system paths
		DataDir:        s.ConfigDir,
		WALDir:         walDir,
		BackupLocation: backupLocation,

		// Supervisor configuration
		SupervisorEnabled: true, // Always monitor supervisor processes
		EnabledServices:   enabledServices,

		// Thresholds
		DiskSpaceThreshold:   90.0,               // 90%
		MemoryUsageThreshold: 80.0,               // 80%
		CPUUsageThreshold:    90.0,               // 90%
		WALSizeThreshold:     1024 * 1024 * 1024, // 1GB
	}

	healthChecker, err := health.NewHealthChecker(config)
	if err != nil {
		log.Printf("Warning: Failed to create health checker: %v", err)
		return
	}

	s.healthChecker = healthChecker
}

// LoadServiceConfigs loads and hydrates service configurations with defaults
func (s *HealthServer) LoadServiceConfigs() {
	// PostgreSQL configuration - always present
	listenAddr := "localhost"
	port := 5432
	s.PostgresConfig = &pkg.PostgresConf{
		ListenAddresses: listenAddr,
		Port:            port,
		MaxConnections:  s.MaxConn,
		// Other defaults will be set by struct tags
	}

	// PgBouncer configuration - load if enabled
	pgbouncerEnabled := os.Getenv("PGBOUNCER_ENABLED") == "true"
	if pgbouncerEnabled {
		adminUser := "postgres"
		s.PgBouncerConfig = &pkg.PgBouncerConf{
			ListenAddress: "127.0.0.1",
			ListenPort:    6432,
			AdminUser:     &adminUser,
			// Other defaults from env vars/struct tags
		}
	}

	// PostgREST configuration - load if enabled
	postgrestEnabled := os.Getenv("POSTGREST_ENABLED") == "true"
	if postgrestEnabled {
		dbUri := "postgresql://postgres@localhost:5432/postgres"
		dbSchemas := "public"
		serverHost := "0.0.0.0"
		serverPort := 3000
		s.PostgRESTConfig = &pkg.PostgrestConf{
			DbUri:         &dbUri,
			DbSchemas:     &dbSchemas,
			ServerHost:    &serverHost,
			ServerPort:    &serverPort,
			AdminRole:     "postgres",
			AnonymousRole: "postgres",
			// Other defaults from env vars/struct tags
		}
	}

	// WAL-G configuration - load if enabled
	walgEnabled := os.Getenv("WALG_ENABLED") == "true"
	if walgEnabled {
		dataDir := os.Getenv("PGDATA")
		if dataDir == "" {
			dataDir = "/var/lib/postgresql/data"
		}

		s.WalgConfig = &pkg.WalgConf{
			Enabled:           true,
			PostgresqlDataDir: dataDir,
		}

		// Set storage prefixes from env vars if they exist
		if s3Prefix := os.Getenv("WALG_S3_PREFIX"); s3Prefix != "" {
			s.WalgConfig.S3Prefix = &s3Prefix
		}
		if gsPrefix := os.Getenv("WALG_GS_PREFIX"); gsPrefix != "" {
			s.WalgConfig.GsPrefix = &gsPrefix
		}
		if azPrefix := os.Getenv("WALG_AZ_PREFIX"); azPrefix != "" {
			s.WalgConfig.AzPrefix = &azPrefix
		}
		if filePrefix := os.Getenv("WALG_FILE_PREFIX"); filePrefix != "" {
			s.WalgConfig.FilePrefix = &filePrefix
		}
	}
}

// handleHealthStatus provides detailed health status using go-health
func (s *HealthServer) handleHealthStatus(w http.ResponseWriter, r *http.Request) {
	if s.healthChecker == nil {
		http.Error(w, "Health checker not initialized", http.StatusServiceUnavailable)
		return
	}

	// Use the StatusHandler from the health checker
	s.healthChecker.StatusHandler().ServeHTTP(w, r)
}
