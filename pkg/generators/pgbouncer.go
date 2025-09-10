package generators

import (
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/postgres/pkg"
	"github.com/flanksource/postgres/pkg/pgtune"
	"github.com/flanksource/postgres/pkg/sysinfo"
)

// PgBouncerConfigGenerator generates pgbouncer.ini configuration
type PgBouncerConfigGenerator struct {
	SystemInfo        *sysinfo.SystemInfo
	TunedParams       *pgtune.TunedParameters
	DatabaseName      string
	DatabaseHost      string
	DatabasePort      int
	DatabaseUser      string
	CustomSettings    map[string]string
	DatabaseOverrides map[string]pkg.DatabaseConfig
	config            *pkg.PgBouncerIni // Store generated config for template use
}

// NewPgBouncerConfigGenerator creates a new PgBouncer configuration generator
func NewPgBouncerConfigGenerator(sysInfo *sysinfo.SystemInfo, params *pgtune.TunedParameters) *PgBouncerConfigGenerator {
	return &PgBouncerConfigGenerator{
		SystemInfo:        sysInfo,
		TunedParams:       params,
		DatabaseName:      "postgres",
		DatabaseHost:      "localhost",
		DatabasePort:      5432,
		DatabaseUser:      "postgres",
		CustomSettings:    make(map[string]string),
		DatabaseOverrides: make(map[string]pkg.DatabaseConfig),
	}
}

// GenerateConfig generates a PgBouncer configuration struct
func (g *PgBouncerConfigGenerator) GenerateConfig() *pkg.PgBouncerIni {
	// Calculate pool sizes based on max_connections
	defaultPoolSize := calculateDefaultPoolSize(g.TunedParams.MaxConnections)
	maxClientConn := calculateMaxClientConn(g.TunedParams.MaxConnections)

	// Helper function to create string pointers
	stringPtr := func(s string) *string { return &s }
	durationPtr := func(d string) *pkg.Duration { dur := pkg.Duration(d); return &dur }

	reservePoolSize := defaultPoolSize / 4 // 25% of default pool size
	zeroDB := 0
	zeroUser := 0
	loginRetry := 1

	authQuery := "SELECT usename, passwd FROM pg_shadow WHERE usename=$1"

	config := &pkg.PgBouncerIni{
		// Connection settings
		ListenAddr:    "0.0.0.0",
		ListenPort:    6432,
		UnixSocketDir: "/tmp",

		// Authentication settings
		AuthType:    "md5",
		AuthFile:    "userlist.txt",
		AuthHbaFile: nil,
		AuthQuery:   &authQuery,

		// Pool settings
		PoolMode:           "transaction",
		DefaultPoolSize:    defaultPoolSize,
		MinPoolSize:        0,
		ReservePoolSize:    &reservePoolSize,
		ReservePoolTimeout: stringPtr("5s"),
		MaxClientConn:      &maxClientConn,
		MaxDbConnections:   &zeroDB, // unlimited
		MaxUserConnections: &zeroUser, // unlimited

		// Timeout settings
		ServerLifetime:       durationPtr("3600s"), // 1 hour
		ServerIdleTimeout:    durationPtr("600s"),  // 10 minutes
		ServerConnectTimeout: durationPtr("15s"),
		ServerLoginRetry:     &loginRetry,
		QueryTimeout:         stringPtr("0"), // disabled
		QueryWaitTimeout:     durationPtr("120s"),
		ClientIdleTimeout:    stringPtr("0"), // disabled
		ClientLoginTimeout:   durationPtr("60s"),

		// Database configurations
		Databases: make(map[string]pkg.DatabaseConfig),
	}

	// Add default database configuration
	host := g.DatabaseHost
	port := g.DatabasePort
	dbname := g.DatabaseName
	user := g.DatabaseUser
	password := ""
	poolSize := defaultPoolSize
	minPoolSize := 0
	connectQuery := ""
	
	config.Databases[g.DatabaseName] = pkg.DatabaseConfig{
		Host:         &host,
		Port:         &port,
		Dbname:       &dbname,
		User:         &user,
		Password:     &password,
		PoolSize:     &poolSize,
		MinPoolSize:  &minPoolSize,
		ConnectQuery: &connectQuery,
		PoolMode:     nil, // Use default
	}

	// Apply database overrides
	for name, override := range g.DatabaseOverrides {
		config.Databases[name] = override
	}

	return config
}

// GenerateConfigFile generates the actual pgbouncer.ini file content
func (g *PgBouncerConfigGenerator) GenerateConfigFile() string {
	// Generate config first to use in templates
	g.config = g.GenerateConfig()

	var sb strings.Builder

	// File header
	sb.WriteString(g.generateHeader())

	// Databases section
	sb.WriteString(g.generateDatabasesSection())

	// pgbouncer section
	sb.WriteString(g.generatePgBouncerSection())

	return sb.String()
}

func (g *PgBouncerConfigGenerator) generateHeader() string {
	return fmt.Sprintf(`;; PgBouncer configuration file
;; Generated automatically by PgTune
;; System: %s
;; Generated: %s
;;
;; Memory: %.1f GB
;; CPUs: %d
;; PostgreSQL Max Connections: %d
;;
;; This configuration uses transaction pooling for optimal performance
;; with high connection counts while maintaining transaction isolation.
;;

`,
		g.SystemInfo.OSType,
		time.Now().Format("2006-01-02 15:04:05"),
		g.SystemInfo.TotalMemoryGB(),
		g.SystemInfo.CPUCount,
		g.TunedParams.MaxConnections,
	)
}

func (g *PgBouncerConfigGenerator) generateDatabasesSection() string {
	sb := strings.Builder{}
	sb.WriteString("[databases]\n")
	sb.WriteString(";; Database connection settings\n")
	sb.WriteString(";; Format: dbname = host=hostname port=port dbname=database\n\n")

	// Generate database entries
	defaultPoolSize := calculateDefaultPoolSize(g.TunedParams.MaxConnections)

	// Add default database
	sb.WriteString(fmt.Sprintf("%s = host=%s port=%d dbname=%s pool_size=%d\n",
		g.DatabaseName, g.DatabaseHost, g.DatabasePort, g.DatabaseName, defaultPoolSize))

	// Add database overrides
	for name, config := range g.DatabaseOverrides {
		if name == g.DatabaseName {
			continue // Skip default database
		}

		host := "localhost"
		port := 5432
		dbname := name
		
		if config.Host != nil {
			host = *config.Host
		}
		if config.Port != nil {
			port = *config.Port
		}
		if config.Dbname != nil {
			dbname = *config.Dbname
		}
		
		sb.WriteString(fmt.Sprintf("%s = host=%s port=%d dbname=%s",
			name, host, port, dbname))

		if config.User != nil && *config.User != "" {
			sb.WriteString(fmt.Sprintf(" user=%s", *config.User))
		}
		if config.Password != nil && *config.Password != "" {
			sb.WriteString(fmt.Sprintf(" password=%s", *config.Password))
		}
		if config.PoolSize != nil && *config.PoolSize > 0 {
			sb.WriteString(fmt.Sprintf(" pool_size=%d", *config.PoolSize))
		}
		if config.ConnectQuery != nil && *config.ConnectQuery != "" {
			sb.WriteString(fmt.Sprintf(" connect_query='%s'", *config.ConnectQuery))
		}

		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	return sb.String()
}

func (g *PgBouncerConfigGenerator) generatePgBouncerSection() string {
	sb := strings.Builder{}
	sb.WriteString("[pgbouncer]\n")
	sb.WriteString(";; PgBouncer settings\n\n")

	// Connection settings
	sb.WriteString(";; Connection Settings\n")
	sb.WriteString(fmt.Sprintf("listen_addr = %s\n", g.config.ListenAddr))
	sb.WriteString(fmt.Sprintf("listen_port = %d\n", g.config.ListenPort))
	sb.WriteString(fmt.Sprintf("unix_socket_dir = %s\n\n", g.config.UnixSocketDir))

	// Pool settings
	sb.WriteString(";; Pool Settings\n")
	sb.WriteString(fmt.Sprintf("pool_mode = %s\n", g.config.PoolMode))
	sb.WriteString(fmt.Sprintf("default_pool_size = %d\n", g.config.DefaultPoolSize))
	sb.WriteString(fmt.Sprintf("min_pool_size = %d\n", g.config.MinPoolSize))
	if g.config.ReservePoolSize != nil {
		sb.WriteString(fmt.Sprintf("reserve_pool_size = %d\n", *g.config.ReservePoolSize))
	}
	if g.config.ReservePoolTimeout != nil {
		sb.WriteString(fmt.Sprintf("reserve_pool_timeout = %s\n", *g.config.ReservePoolTimeout))
	}
	if g.config.MaxClientConn != nil {
		sb.WriteString(fmt.Sprintf("max_client_conn = %d\n", *g.config.MaxClientConn))
	}
	if g.config.MaxDbConnections != nil {
		sb.WriteString(fmt.Sprintf("max_db_connections = %d\n", *g.config.MaxDbConnections))
	}
	if g.config.MaxUserConnections != nil {
		sb.WriteString(fmt.Sprintf("max_user_connections = %d\n", *g.config.MaxUserConnections))
	}
	sb.WriteString("\n")

	// Authentication settings
	sb.WriteString(";; Authentication\n")
	sb.WriteString(fmt.Sprintf("auth_type = %s\n", g.config.AuthType))
	sb.WriteString(fmt.Sprintf("auth_file = %s\n", g.config.AuthFile))
	if g.config.AuthHbaFile != nil && *g.config.AuthHbaFile != "" {
		sb.WriteString(fmt.Sprintf("auth_hba_file = %s\n", *g.config.AuthHbaFile))
	} else {
		sb.WriteString(";auth_hba_file = pg_hba.conf\n")
	}
	if g.config.AuthQuery != nil {
		sb.WriteString(fmt.Sprintf("auth_query = %s\n", *g.config.AuthQuery))
	}
	sb.WriteString("\n")

	// Timeout settings (convert from Duration to seconds for ini format)
	sb.WriteString(";; Timeouts\n")
	if g.config.ServerLifetime != nil {
		sb.WriteString(fmt.Sprintf("server_lifetime = %s\n", g.extractSecondsFromDuration(*g.config.ServerLifetime)))
	}
	if g.config.ServerIdleTimeout != nil {
		sb.WriteString(fmt.Sprintf("server_idle_timeout = %s\n", g.extractSecondsFromDuration(*g.config.ServerIdleTimeout)))
	}
	if g.config.ServerConnectTimeout != nil {
		sb.WriteString(fmt.Sprintf("server_connect_timeout = %s\n", g.extractSecondsFromDuration(*g.config.ServerConnectTimeout)))
	}
	if g.config.ServerLoginRetry != nil {
		sb.WriteString(fmt.Sprintf("server_login_retry = %d\n", *g.config.ServerLoginRetry))
	}
	if g.config.QueryTimeout != nil {
		sb.WriteString(fmt.Sprintf("query_timeout = %s\n", *g.config.QueryTimeout))
	}
	if g.config.QueryWaitTimeout != nil {
		sb.WriteString(fmt.Sprintf("query_wait_timeout = %s\n", g.extractSecondsFromDuration(*g.config.QueryWaitTimeout)))
	}
	if g.config.ClientIdleTimeout != nil {
		sb.WriteString(fmt.Sprintf("client_idle_timeout = %s\n", *g.config.ClientIdleTimeout))
	}
	if g.config.ClientLoginTimeout != nil {
		sb.WriteString(fmt.Sprintf("client_login_timeout = %s\n", g.extractSecondsFromDuration(*g.config.ClientLoginTimeout)))
	}
	sb.WriteString("\n")

	// End of configuration
	sb.WriteString(";; Additional settings can be added as needed\n")

	// Add custom settings if any
	if len(g.CustomSettings) > 0 {
		sb.WriteString(";; Custom Settings\n")
		for key, value := range g.CustomSettings {
			sb.WriteString(fmt.Sprintf("%s = %s\n", key, value))
		}
	}


	sb.WriteString("\n")
	return sb.String()
}

// SetDatabaseConfig sets the default database configuration
func (g *PgBouncerConfigGenerator) SetDatabaseConfig(name, host string, port int, dbname, user string) {
	g.DatabaseName = name
	g.DatabaseHost = host
	g.DatabasePort = port
	g.DatabaseUser = user

	if dbname == "" {
		dbname = name
	}

	g.DatabaseOverrides[name] = pkg.DatabaseConfig{
		Host:   &host,
		Port:   &port,
		Dbname: &dbname,
		User:   &user,
	}
}

// AddDatabase adds a database configuration
func (g *PgBouncerConfigGenerator) AddDatabase(name string, config pkg.DatabaseConfig) {
	g.DatabaseOverrides[name] = config
}

// AddCustomSetting adds a custom setting to the configuration
func (g *PgBouncerConfigGenerator) AddCustomSetting(key, value string) {
	g.CustomSettings[key] = value
}

// calculateDefaultPoolSize calculates appropriate default pool size based on max_connections
func calculateDefaultPoolSize(maxConnections int) int {
	// Use a fraction of max_connections as the default pool size
	// This allows for connection pooling benefits while ensuring we don't exhaust connections
	poolSize := maxConnections / 4

	// Minimum pool size
	if poolSize < 5 {
		poolSize = 5
	}

	// Maximum pool size
	if poolSize > 50 {
		poolSize = 50
	}

	return poolSize
}

// calculateMaxClientConn calculates maximum client connections
func calculateMaxClientConn(maxConnections int) int {
	// Allow more client connections than database connections due to pooling
	maxClient := maxConnections * 2

	// Reasonable minimum
	if maxClient < 100 {
		maxClient = 100
	}

	// Reasonable maximum
	if maxClient > 1000 {
		maxClient = 1000
	}

	return maxClient
}

// Helper function to convert boolean to int (0/1) for pgbouncer ini format
func (g *PgBouncerConfigGenerator) boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Helper function to extract seconds from Duration string (e.g., "600s" -> "600")
func (g *PgBouncerConfigGenerator) extractSeconds(duration string) string {
	if duration == "" || duration == "0" {
		return "0"
	}
	// Simple extraction - assumes format like "600s"
	if strings.HasSuffix(duration, "s") {
		return strings.TrimSuffix(duration, "s")
	}
	return duration
}

// Helper function to extract seconds from pkg.Duration type
func (g *PgBouncerConfigGenerator) extractSecondsFromDuration(duration pkg.Duration) string {
	return g.extractSeconds(string(duration))
}
