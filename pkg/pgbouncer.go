package pkg

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// PgBouncer represents a PgBouncer connection pooler service instance
type PgBouncer struct {
	Config *PgBouncerConf
}

// NewPgBouncer creates a new PgBouncer service instance
func NewPgBouncer(config *PgBouncerConf) *PgBouncer {
	return &PgBouncer{
		Config: config,
	}
}

// Health performs a comprehensive health check of the PgBouncer service
func (p *PgBouncer) Health() error {
	if p == nil {
		return fmt.Errorf("PgBouncer service is nil")
	}
	if p.Config == nil {
		return fmt.Errorf("PgBouncer configuration not provided")
	}

	// Build connection string for PgBouncer admin database
	adminUser := ""
	if p.Config.AdminUser != nil {
		adminUser = *p.Config.AdminUser
	}
	connStr := fmt.Sprintf("host=%s port=%d dbname=pgbouncer user=%s sslmode=disable",
		p.Config.ListenAddress, p.Config.ListenPort, adminUser)

	// Add password if configured
	if p.Config.AdminPassword != nil && *p.Config.AdminPassword != "" {
		connStr += " password=" + *p.Config.AdminPassword
	}

	// Connect to PgBouncer admin database
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to connect to PgBouncer admin database: %w", err)
	}
	defer db.Close()

	// Set connection timeouts
	db.SetConnMaxLifetime(time.Minute * 1)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Test basic connectivity
	if err := db.Ping(); err != nil {
		return fmt.Errorf("PgBouncer ping failed: %w", err)
	}

	// Check if we can execute SHOW POOLS command
	_, err = db.Query("SHOW POOLS")
	if err != nil {
		return fmt.Errorf("failed to execute SHOW POOLS: %w", err)
	}

	return nil
}

// ShowPools retrieves information about connection pools
func (p *PgBouncer) ShowPools() ([]PoolInfo, error) {
	if p.Config == nil {
		return nil, fmt.Errorf("PgBouncer configuration not provided")
	}

	adminUser2 := ""
	if p.Config.AdminUser != nil {
		adminUser2 = *p.Config.AdminUser
	}
	connStr := fmt.Sprintf("host=%s port=%d dbname=pgbouncer user=%s sslmode=disable",
		p.Config.ListenAddress, p.Config.ListenPort, adminUser2)

	if p.Config.AdminPassword != nil && *p.Config.AdminPassword != "" {
		connStr += " password=" + *p.Config.AdminPassword
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PgBouncer: %w", err)
	}
	defer db.Close()

	rows, err := db.Query("SHOW POOLS")
	if err != nil {
		return nil, fmt.Errorf("failed to execute SHOW POOLS: %w", err)
	}
	defer rows.Close()

	var pools []PoolInfo
	for rows.Next() {
		var pool PoolInfo
		err := rows.Scan(
			&pool.Database, &pool.User, &pool.ClientActive, &pool.ClientWaiting,
			&pool.ServerActive, &pool.ServerIdle, &pool.ServerUsed, &pool.ServerTested,
			&pool.ServerLogin, &pool.MaxWait, &pool.MaxWaitUs, &pool.PoolMode,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pool row: %w", err)
		}
		pools = append(pools, pool)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating pool rows: %w", err)
	}

	return pools, nil
}

// ShowStats retrieves PgBouncer statistics
func (p *PgBouncer) ShowStats() (*Stats, error) {
	if p.Config == nil {
		return nil, fmt.Errorf("PgBouncer configuration not provided")
	}

	adminUser3 := ""
	if p.Config.AdminUser != nil {
		adminUser3 = *p.Config.AdminUser
	}
	connStr := fmt.Sprintf("host=%s port=%d dbname=pgbouncer user=%s sslmode=disable",
		p.Config.ListenAddress, p.Config.ListenPort, adminUser3)

	if p.Config.AdminPassword != nil && *p.Config.AdminPassword != "" {
		connStr += " password=" + *p.Config.AdminPassword
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PgBouncer: %w", err)
	}
	defer db.Close()

	rows, err := db.Query("SHOW STATS")
	if err != nil {
		return nil, fmt.Errorf("failed to execute SHOW STATS: %w", err)
	}
	defer rows.Close()

	var stats Stats
	if rows.Next() {
		err := rows.Scan(
			&stats.Database, &stats.TotalXactCount, &stats.TotalQueryCount,
			&stats.TotalReceived, &stats.TotalSent, &stats.TotalXactTime,
			&stats.TotalQueryTime, &stats.TotalWaitTime, &stats.AvgXactCount,
			&stats.AvgQueryCount, &stats.AvgReceived, &stats.AvgSent,
			&stats.AvgXactTime, &stats.AvgQueryTime, &stats.AvgWaitTime,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan stats row: %w", err)
		}
	}

	return &stats, nil
}

// GetStatus returns detailed PgBouncer service status
func (p *PgBouncer) GetStatus() (*PgBouncerStatus, error) {
	adminUserStatus := ""
	if p.Config.AdminUser != nil {
		adminUserStatus = *p.Config.AdminUser
	}
	status := &PgBouncerStatus{
		Address:   fmt.Sprintf("%s:%d", p.Config.ListenAddress, p.Config.ListenPort),
		Healthy:   false,
		CheckTime: time.Now(),
		AdminUser: adminUserStatus,
	}

	// Perform health check
	if err := p.Health(); err != nil {
		status.Error = err.Error()
		return status, nil
	}

	status.Healthy = true

	// Get pool information
	pools, err := p.ShowPools()
	if err != nil {
		status.Error = fmt.Sprintf("Failed to get pool info: %v", err)
		return status, nil
	}
	status.PoolCount = len(pools)

	// Calculate totals
	for _, pool := range pools {
		status.TotalClients += pool.ClientActive + pool.ClientWaiting
		status.TotalServers += pool.ServerActive + pool.ServerIdle + pool.ServerUsed
	}

	return status, nil
}

// PoolInfo represents information about a connection pool
type PoolInfo struct {
	Database      string `json:"database"`
	User          string `json:"user"`
	ClientActive  int    `json:"client_active"`
	ClientWaiting int    `json:"client_waiting"`
	ServerActive  int    `json:"server_active"`
	ServerIdle    int    `json:"server_idle"`
	ServerUsed    int    `json:"server_used"`
	ServerTested  int    `json:"server_tested"`
	ServerLogin   int    `json:"server_login"`
	MaxWait       int    `json:"max_wait"`
	MaxWaitUs     int    `json:"max_wait_us"`
	PoolMode      string `json:"pool_mode"`
}

// Stats represents PgBouncer statistics
type Stats struct {
	Database        string `json:"database"`
	TotalXactCount  int64  `json:"total_xact_count"`
	TotalQueryCount int64  `json:"total_query_count"`
	TotalReceived   int64  `json:"total_received"`
	TotalSent       int64  `json:"total_sent"`
	TotalXactTime   int64  `json:"total_xact_time"`
	TotalQueryTime  int64  `json:"total_query_time"`
	TotalWaitTime   int64  `json:"total_wait_time"`
	AvgXactCount    int64  `json:"avg_xact_count"`
	AvgQueryCount   int64  `json:"avg_query_count"`
	AvgReceived     int64  `json:"avg_received"`
	AvgSent         int64  `json:"avg_sent"`
	AvgXactTime     int64  `json:"avg_xact_time"`
	AvgQueryTime    int64  `json:"avg_query_time"`
	AvgWaitTime     int64  `json:"avg_wait_time"`
}

// PgBouncerStatus represents the status of a PgBouncer service
type PgBouncerStatus struct {
	Address      string    `json:"address"`
	Healthy      bool      `json:"healthy"`
	CheckTime    time.Time `json:"check_time"`
	AdminUser    string    `json:"admin_user"`
	PoolCount    int       `json:"pool_count"`
	TotalClients int       `json:"total_clients"`
	TotalServers int       `json:"total_servers"`
	Error        string    `json:"error,omitempty"`
}

// Validate validates a PgBouncer configuration file using the pgbouncer binary
func (p *PgBouncer) Validate(config []byte) error {
	// Create a temporary file for the config
	tempFile, err := ioutil.TempFile("", "pgbouncer_validate_*.ini")
	if err != nil {
		return fmt.Errorf("failed to create temp config file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	// Write config to temp file
	if _, err := tempFile.Write(config); err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to write config to temp file: %w", err)
	}
	tempFile.Close()

	// First, perform basic INI syntax validation
	if err := p.validateINISyntax(config); err != nil {
		return err
	}

	// Try to validate using pgbouncer binary if available
	if err := p.validateWithBinary(tempFile.Name()); err != nil {
		// If binary validation fails, at least we did INI syntax validation
		return err
	}

	return nil
}

// validateINISyntax performs basic INI file syntax validation
func (p *PgBouncer) validateINISyntax(config []byte) error {
	lines := strings.Split(string(config), "\n")
	var currentSection string
	foundSections := make(map[string]bool)

	for lineNum, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}

		// Check for section headers
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.Trim(line, "[]")
			foundSections[currentSection] = true
			continue
		}

		// Check for key=value pairs
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				return &ValidationError{
					Line:    lineNum + 1,
					Message: "invalid key=value format",
					Raw:     line,
				}
			}

			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			if key == "" {
				return &ValidationError{
					Line:      lineNum + 1,
					Parameter: key,
					Message:   "empty parameter name",
					Raw:       line,
				}
			}

			// Validate database connection strings in [databases] section
			if currentSection == "databases" {
				if err := p.validateDatabaseConnectionString(key, value, lineNum+1); err != nil {
					return err
				}
			}

			continue
		}

		// If we get here, it's an invalid line
		return &ValidationError{
			Line:    lineNum + 1,
			Message: "invalid line format - expected section header or key=value pair",
			Raw:     line,
		}
	}

	// Check for required sections
	if !foundSections["databases"] {
		return &ValidationError{
			Message: "missing required [databases] section",
		}
	}

	if !foundSections["pgbouncer"] {
		return &ValidationError{
			Message: "missing required [pgbouncer] section",
		}
	}

	return nil
}

// validateDatabaseConnectionString validates database connection string format
func (p *PgBouncer) validateDatabaseConnectionString(dbName, connStr string, lineNum int) error {
	// Basic validation of connection string format
	// Should contain key=value pairs separated by spaces

	if connStr == "" {
		return &ValidationError{
			Line:      lineNum,
			Parameter: dbName,
			Message:   "empty connection string",
		}
	}

	// Check for required components (at least host and dbname)
	hasHost := strings.Contains(connStr, "host=")
	hasDBName := strings.Contains(connStr, "dbname=")

	if !hasHost {
		return &ValidationError{
			Line:      lineNum,
			Parameter: dbName,
			Message:   "connection string missing 'host=' parameter",
			Raw:       connStr,
		}
	}

	if !hasDBName {
		return &ValidationError{
			Line:      lineNum,
			Parameter: dbName,
			Message:   "connection string missing 'dbname=' parameter",
			Raw:       connStr,
		}
	}

	return nil
}

// validateWithBinary attempts to validate using the pgbouncer binary
func (p *PgBouncer) validateWithBinary(configPath string) error {
	// Try to run pgbouncer with a test configuration
	// Use -d flag to run in daemon mode with invalid pidfile to test config loading
	cmd := exec.Command("pgbouncer", "-d", configPath)

	// Set environment to avoid actual startup
	cmd.Env = append(os.Environ(),
		"PGBOUNCER_PIDFILE=/tmp/pgbouncer_validate_invalid.pid", // Invalid path to prevent actual startup
		"PGBOUNCER_LOGFILE=/dev/null",
	)

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		return p.parsePgBouncerValidationError(outputStr, err)
	}

	// If no error, configuration is valid
	return nil
}

// parsePgBouncerValidationError parses PgBouncer validation errors and returns structured error
func (p *PgBouncer) parsePgBouncerValidationError(output string, originalErr error) error {
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse different error patterns
		if strings.Contains(line, "unknown parameter") {
			// Example: FATAL unknown parameter: invalid_param
			re := regexp.MustCompile(`unknown parameter:?\s*([^\s]+)`)
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				return &ValidationError{
					Parameter: matches[1],
					Message:   "unknown parameter",
					Raw:       line,
				}
			}
		}

		if strings.Contains(line, "invalid value") {
			// Example: FATAL invalid value for parameter max_client_conn: invalid
			re := regexp.MustCompile(`invalid value for parameter ([^:]+):\s*(.+)`)
			if matches := re.FindStringSubmatch(line); len(matches) > 2 {
				return &ValidationError{
					Parameter: strings.TrimSpace(matches[1]),
					Message:   fmt.Sprintf("invalid value: %s", strings.TrimSpace(matches[2])),
					Raw:       line,
				}
			}
		}

		if strings.Contains(line, "config file") && strings.Contains(line, "error") {
			return &ValidationError{
				Message: "configuration file contains errors",
				Raw:     line,
			}
		}

		// Check for missing database configuration
		if strings.Contains(line, "no databases defined") {
			return &ValidationError{
				Message: "no databases defined in [databases] section",
				Raw:     line,
			}
		}
	}

	// If we couldn't parse a specific error, return a general one
	return &ValidationError{
		Message: fmt.Sprintf("configuration validation failed: %s", strings.TrimSpace(output)),
		Raw:     output,
	}
}
