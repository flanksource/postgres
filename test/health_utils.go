package test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	_ "github.com/lib/pq"
)

// HealthCheckConfig holds configuration for health checking
type HealthCheckConfig struct {
	PostgreSQLHost string
	PostgreSQLPort int
	PgBouncerHost  string
	PgBouncerPort  int
	PostgRESTHost  string
	PostgRESTPort  int
	User           string
	Password       string
	Database       string
	Timeout        time.Duration
}

// DefaultHealthCheckConfig returns the default health check configuration
func DefaultHealthCheckConfig() HealthCheckConfig {
	return HealthCheckConfig{
		PostgreSQLHost: "localhost",
		PostgreSQLPort: 5432,
		PgBouncerHost:  "localhost",
		PgBouncerPort:  6432,
		PostgRESTHost:  "localhost",
		PostgRESTPort:  3000,
		User:           "postgres",
		Password:       "postgres",
		Database:       "postgres",
		Timeout:        10 * time.Second,
	}
}

// HealthStatus represents the health status of a service
type HealthStatus struct {
	Service   string                 `json:"service"`
	Healthy   bool                   `json:"healthy"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	CheckedAt time.Time              `json:"checked_at"`
	Duration  time.Duration          `json:"duration"`
}

// HealthChecker provides comprehensive health checking for all services
type HealthChecker struct {
	config HealthCheckConfig
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(config HealthCheckConfig) *HealthChecker {
	return &HealthChecker{config: config}
}

// CheckPostgreSQLHealth checks the health of the PostgreSQL service
func (hc *HealthChecker) CheckPostgreSQLHealth() HealthStatus {
	start := time.Now()
	status := HealthStatus{
		Service:   "PostgreSQL",
		CheckedAt: start,
		Details:   make(map[string]interface{}),
	}

	ctx, cancel := context.WithTimeout(context.Background(), hc.config.Timeout)
	defer cancel()

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		hc.config.PostgreSQLHost, hc.config.PostgreSQLPort, hc.config.User, hc.config.Password, hc.config.Database)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		status.Healthy = false
		status.Message = fmt.Sprintf("Failed to open connection: %v", err)
		status.Duration = time.Since(start)
		return status
	}
	defer db.Close()

	// Test connection
	if err := db.PingContext(ctx); err != nil {
		status.Healthy = false
		status.Message = fmt.Sprintf("Failed to ping database: %v", err)
		status.Duration = time.Since(start)
		return status
	}

	// Get version information
	var version string
	err = db.QueryRowContext(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		status.Healthy = false
		status.Message = fmt.Sprintf("Failed to get version: %v", err)
		status.Duration = time.Since(start)
		return status
	}

	// Get connection count
	var connections int
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM pg_stat_activity").Scan(&connections)
	if err != nil {
		status.Healthy = false
		status.Message = fmt.Sprintf("Failed to get connection count: %v", err)
		status.Duration = time.Since(start)
		return status
	}

	// Get database size
	var size string
	err = db.QueryRowContext(ctx, "SELECT pg_size_pretty(pg_database_size(current_database()))").Scan(&size)
	if err != nil {
		status.Healthy = false
		status.Message = fmt.Sprintf("Failed to get database size: %v", err)
		status.Duration = time.Since(start)
		return status
	}

	status.Healthy = true
	status.Message = "PostgreSQL is healthy"
	status.Details["version"] = version
	status.Details["active_connections"] = connections
	status.Details["database_size"] = size
	status.Duration = time.Since(start)

	return status
}

// CheckPgBouncerHealth checks the health of the PgBouncer service
func (hc *HealthChecker) CheckPgBouncerHealth() HealthStatus {
	start := time.Now()
	status := HealthStatus{
		Service:   "PgBouncer",
		CheckedAt: start,
		Details:   make(map[string]interface{}),
	}

	ctx, cancel := context.WithTimeout(context.Background(), hc.config.Timeout)
	defer cancel()

	// Test connection through PgBouncer
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		hc.config.PgBouncerHost, hc.config.PgBouncerPort, hc.config.User, hc.config.Password, hc.config.Database)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		status.Healthy = false
		status.Message = fmt.Sprintf("Failed to connect through PgBouncer: %v", err)
		status.Duration = time.Since(start)
		return status
	}
	defer db.Close()

	// Test basic query
	var result int
	err = db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		status.Healthy = false
		status.Message = fmt.Sprintf("Failed to execute query through PgBouncer: %v", err)
		status.Duration = time.Since(start)
		return status
	}

	// Get PgBouncer statistics (connect to pgbouncer admin database)
	adminConnStr := fmt.Sprintf("host=%s port=%d user=%s dbname=pgbouncer sslmode=disable",
		hc.config.PgBouncerHost, hc.config.PgBouncerPort, hc.config.User)

	adminDb, err := sql.Open("postgres", adminConnStr)
	if err != nil {
		status.Healthy = false
		status.Message = fmt.Sprintf("Failed to connect to PgBouncer admin: %v", err)
		status.Duration = time.Since(start)
		return status
	}
	defer adminDb.Close()

	// Get pool information
	pools := make(map[string]map[string]interface{})
	rows, err := adminDb.QueryContext(ctx, "SHOW POOLS")
	if err != nil {
		status.Healthy = false
		status.Message = fmt.Sprintf("Failed to get pool information: %v", err)
		status.Duration = time.Since(start)
		return status
	}
	defer rows.Close()

	for rows.Next() {
		var database, user, clActive, clWaiting, svActive, svIdle, svUsed, svTested, svLogin, maxwait, maxwaitUs, poolMode string
		err := rows.Scan(&database, &user, &clActive, &clWaiting, &svActive, &svIdle, &svUsed, &svTested, &svLogin, &maxwait, &maxwaitUs, &poolMode)
		if err != nil {
			continue
		}

		pools[database] = map[string]interface{}{
			"user":       user,
			"cl_active":  clActive,
			"cl_waiting": clWaiting,
			"sv_active":  svActive,
			"sv_idle":    svIdle,
			"pool_mode":  poolMode,
		}
	}

	status.Healthy = true
	status.Message = "PgBouncer is healthy"
	status.Details["pools"] = pools
	status.Duration = time.Since(start)

	return status
}

// CheckPostgRESTHealth checks the health of the PostgREST service
func (hc *HealthChecker) CheckPostgRESTHealth() HealthStatus {
	start := time.Now()
	status := HealthStatus{
		Service:   "PostgREST",
		CheckedAt: start,
		Details:   make(map[string]interface{}),
	}

	ctx, cancel := context.WithTimeout(context.Background(), hc.config.Timeout)
	defer cancel()

	client := &http.Client{Timeout: hc.config.Timeout}
	url := fmt.Sprintf("http://%s:%d/", hc.config.PostgRESTHost, hc.config.PostgRESTPort)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		status.Healthy = false
		status.Message = fmt.Sprintf("Failed to create request: %v", err)
		status.Duration = time.Since(start)
		return status
	}

	resp, err := client.Do(req)
	if err != nil {
		status.Healthy = false
		status.Message = fmt.Sprintf("Failed to connect to PostgREST: %v", err)
		status.Duration = time.Since(start)
		return status
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		status.Healthy = false
		status.Message = fmt.Sprintf("PostgREST returned status %d", resp.StatusCode)
		status.Duration = time.Since(start)
		return status
	}

	// Test a specific endpoint (pg_extension table)
	extensionUrl := fmt.Sprintf("http://%s:%d/pg_extension", hc.config.PostgRESTHost, hc.config.PostgRESTPort)
	extReq, err := http.NewRequestWithContext(ctx, "GET", extensionUrl, nil)
	if err == nil {
		extResp, err := client.Do(extReq)
		if err == nil {
			defer extResp.Body.Close()
			status.Details["pg_extension_endpoint"] = map[string]interface{}{
				"status_code": extResp.StatusCode,
				"accessible":  extResp.StatusCode == http.StatusOK,
			}
		}
	}

	status.Healthy = true
	status.Message = "PostgREST is healthy"
	status.Details["status_code"] = resp.StatusCode
	status.Duration = time.Since(start)

	return status
}

// CheckExtensionsHealth checks the health/availability of PostgreSQL extensions
func (hc *HealthChecker) CheckExtensionsHealth() HealthStatus {
	start := time.Now()
	status := HealthStatus{
		Service:   "Extensions",
		CheckedAt: start,
		Details:   make(map[string]interface{}),
	}

	ctx, cancel := context.WithTimeout(context.Background(), hc.config.Timeout)
	defer cancel()

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		hc.config.PostgreSQLHost, hc.config.PostgreSQLPort, hc.config.User, hc.config.Password, hc.config.Database)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		status.Healthy = false
		status.Message = fmt.Sprintf("Failed to connect to database: %v", err)
		status.Duration = time.Since(start)
		return status
	}
	defer db.Close()

	// Get all installed extensions
	extensions := make(map[string]string)
	rows, err := db.QueryContext(ctx, "SELECT extname, extversion FROM pg_extension ORDER BY extname")
	if err != nil {
		status.Healthy = false
		status.Message = fmt.Sprintf("Failed to query extensions: %v", err)
		status.Duration = time.Since(start)
		return status
	}
	defer rows.Close()

	for rows.Next() {
		var name, version string
		if err := rows.Scan(&name, &version); err != nil {
			continue
		}
		extensions[name] = version
	}

	// Check for expected extensions
	expectedExtensions := []string{"vector", "pgsodium", "pgjwt", "pgaudit", "pg_cron"}
	missingExtensions := []string{}
	installedExpected := make(map[string]string)

	for _, expected := range expectedExtensions {
		if version, exists := extensions[expected]; exists {
			installedExpected[expected] = version
		} else {
			missingExtensions = append(missingExtensions, expected)
		}
	}

	status.Healthy = len(missingExtensions) == 0
	if status.Healthy {
		status.Message = fmt.Sprintf("All expected extensions are installed (%d total extensions)", len(extensions))
	} else {
		status.Message = fmt.Sprintf("Missing expected extensions: %v", missingExtensions)
	}

	status.Details["total_extensions"] = len(extensions)
	status.Details["all_extensions"] = extensions
	status.Details["expected_extensions"] = installedExpected
	status.Details["missing_extensions"] = missingExtensions
	status.Duration = time.Since(start)

	return status
}

// CheckAllServices runs health checks on all services
func (hc *HealthChecker) CheckAllServices() map[string]HealthStatus {
	results := make(map[string]HealthStatus)

	// Run health checks in parallel for better performance
	done := make(chan struct {
		string
		HealthStatus
	}, 4)

	go func() {
		done <- struct {
			string
			HealthStatus
		}{"postgresql", hc.CheckPostgreSQLHealth()}
	}()

	go func() {
		done <- struct {
			string
			HealthStatus
		}{"pgbouncer", hc.CheckPgBouncerHealth()}
	}()

	go func() {
		done <- struct {
			string
			HealthStatus
		}{"postgrest", hc.CheckPostgRESTHealth()}
	}()

	go func() {
		done <- struct {
			string
			HealthStatus
		}{"extensions", hc.CheckExtensionsHealth()}
	}()

	// Collect results
	for i := 0; i < 4; i++ {
		result := <-done
		results[result.string] = result.HealthStatus
	}

	return results
}

// IsSystemHealthy returns true if all critical services are healthy
func (hc *HealthChecker) IsSystemHealthy() (bool, map[string]HealthStatus) {
	results := hc.CheckAllServices()

	for _, status := range results {
		if !status.Healthy {
			return false, results
		}
	}

	return true, results
}

// WaitForSystemHealthy waits for all services to become healthy with timeout
func (hc *HealthChecker) WaitForSystemHealthy(maxWait time.Duration) (bool, map[string]HealthStatus) {
	deadline := time.Now().Add(maxWait)

	for time.Now().Before(deadline) {
		if healthy, results := hc.IsSystemHealthy(); healthy {
			return true, results
		}

		// Wait before retrying
		time.Sleep(2 * time.Second)
	}

	// Return final status
	return hc.IsSystemHealthy()
}

// PrintHealthStatus prints a formatted health status report
func PrintHealthStatus(status HealthStatus) {
	icon := "❌"
	if status.Healthy {
		icon = "✅"
	}

	fmt.Printf("%s %s: %s (%.2fs)\n",
		icon, status.Service, status.Message, status.Duration.Seconds())

	if len(status.Details) > 0 {
		for key, value := range status.Details {
			fmt.Printf("   %s: %v\n", key, value)
		}
	}
}

// PrintAllHealthStatuses prints health status for all services
func PrintAllHealthStatuses(statuses map[string]HealthStatus) {
	fmt.Println("🏥 Health Check Results:")
	fmt.Println("========================")

	for _, status := range statuses {
		PrintHealthStatus(status)
		fmt.Println()
	}

	// Summary
	healthy := 0
	total := len(statuses)
	for _, status := range statuses {
		if status.Healthy {
			healthy++
		}
	}

	fmt.Printf("📊 Summary: %d/%d services healthy\n", healthy, total)
}
