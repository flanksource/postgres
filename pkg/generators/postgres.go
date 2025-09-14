package generators

import (
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/postgres/pkg"
	"github.com/flanksource/postgres/pkg/pgtune"
	"github.com/flanksource/postgres/pkg/sysinfo"
	"github.com/flanksource/postgres/pkg/types"
	"github.com/flanksource/postgres/pkg/utils"
)

// PostgreSQLConfigGenerator generates postgresql.conf configuration
type PostgreSQLConfigGenerator struct {
	SystemInfo     *sysinfo.SystemInfo
	TunedParams    *pgtune.TunedParameters
	CustomSettings map[string]string
	config         *pkg.PostgresConf // Store generated config for template use
	extensions     map[string]pkg.ExtensionConfig // Store extensions
	pgauditConf    *pkg.PGAuditConf // Store PGAudit config
}

// NewPostgreSQLConfigGenerator creates a new PostgreSQL configuration generator
func NewPostgreSQLConfigGenerator(sysInfo *sysinfo.SystemInfo, params *pgtune.TunedParameters) *PostgreSQLConfigGenerator {
	return &PostgreSQLConfigGenerator{
		SystemInfo:     sysInfo,
		TunedParams:    params,
		CustomSettings: make(map[string]string),
		extensions:     make(map[string]pkg.ExtensionConfig),
	}
}

// SetExtensions sets the extensions configuration
func (g *PostgreSQLConfigGenerator) SetExtensions(extensions map[string]pkg.ExtensionConfig) {
	g.extensions = extensions
}

// SetPGAuditConf sets the PGAudit configuration
func (g *PostgreSQLConfigGenerator) SetPGAuditConf(conf *pkg.PGAuditConf) {
	g.pgauditConf = conf
}

// GenerateConfig generates a PostgreSQL configuration struct
func (g *PostgreSQLConfigGenerator) GenerateConfig() *pkg.PostgresConf {
	// Helper functions to convert to pointers
	intPtr := func(i int) *int { return &i }
	strPtr := func(s string) *string { return &s }
	sizePtr := func(kb uint64) *types.Size { val := types.Size(utils.KBToBytes(kb)); return &val }
	
	config := &pkg.PostgresConf{
		// Connection settings (using actual field names from schema)
		Port:           intPtr(5432),
		MaxConnections: intPtr(g.TunedParams.MaxConnections),

		// Memory settings (convert KB values to types.Size values)
		SharedBuffers: sizePtr(g.TunedParams.SharedBuffers),
		WorkMem:       sizePtr(g.TunedParams.WorkMem),

		// Security settings
		PasswordEncryption: "md5",
		
		// SSL settings
		SslCertFile: strPtr("/etc/ssl/certs/server.crt"),
		SslKeyFile:  strPtr("/etc/ssl/private/server.key"),
		
		// Logging settings
		LogStatement: "none",
	}

	return config
}

// GenerateConfigFile generates the actual postgresql.conf file content
func (g *PostgreSQLConfigGenerator) GenerateConfigFile() string {
	// Generate config first to use in templates
	g.config = g.GenerateConfig()

	var sb strings.Builder

	// File header
	sb.WriteString(g.generateHeader())

	// Connection and authentication
	sb.WriteString(g.generateConnectionSection())

	// Resource usage
	sb.WriteString(g.generateResourceSection())

	// Write-Ahead Log
	sb.WriteString(g.generateWALSection())

	// Query tuning
	sb.WriteString(g.generateQueryTuningSection())

	// Parallel processing
	sb.WriteString(g.generateParallelSection())

	// Logging
	sb.WriteString(g.generateLoggingSection())

	// SSL/TLS
	sb.WriteString(g.generateSSLSection())

	// Extensions
	sb.WriteString(g.generateExtensionSection())

	// Custom settings
	if len(g.CustomSettings) > 0 {
		sb.WriteString(g.generateCustomSection())
	}

	// Warnings
	if len(g.TunedParams.Warnings) > 0 {
		sb.WriteString(g.generateWarningsSection())
	}

	return sb.String()
}

func (g *PostgreSQLConfigGenerator) generateHeader() string {
	return fmt.Sprintf(`# -----------------------------
# PostgreSQL configuration file
# -----------------------------
#
# This file was generated automatically by PgTune
# System: %s
# Generated: %s
#
# Memory: %.1f GB
# CPUs: %d
# PostgreSQL Version: %.1f
# Max Connections: %d
# Workload Type: %s
# Disk Type: %s
#

`,
		g.SystemInfo.OSType,
		time.Now().Format("2006-01-02 15:04:05"),
		g.SystemInfo.TotalMemoryGB(),
		g.SystemInfo.CPUCount,
		g.SystemInfo.PostgreSQLVersion,
		g.TunedParams.MaxConnections,
		inferDBType(g.TunedParams),
		g.SystemInfo.DiskType,
	)
}

func (g *PostgreSQLConfigGenerator) generateConnectionSection() string {
	var sb strings.Builder
	
	sb.WriteString(`# -----------------------------
# CONNECTIONS AND AUTHENTICATION
# -----------------------------

`)

	// Listen addresses with detailed description
	sb.WriteString(FormatConfigComment("postgres.listen_addresses", "#"))
	sb.WriteString("\n")
	listenAddr := "localhost"
	if g.config.ListenAddresses != nil {
		listenAddr = *g.config.ListenAddresses
	}
	sb.WriteString(fmt.Sprintf("listen_addresses = '%s'\n", listenAddr))
	sb.WriteString("\n")

	// Port with description
	sb.WriteString(FormatConfigComment("postgres.port", "#"))
	sb.WriteString("\n")
	port := 5432
	if g.config.Port != nil {
		port = *g.config.Port
	}
	sb.WriteString(fmt.Sprintf("port = %d\n", port))
	sb.WriteString("\n")

	// Max connections with description
	sb.WriteString(FormatConfigComment("postgres.max_connections", "#"))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("max_connections = %d\n", g.config.MaxConnections))
	sb.WriteString("\n")

	// Password encryption with description
	sb.WriteString(FormatConfigComment("postgres.password_encryption", "#"))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("password_encryption = %s\n", g.config.PasswordEncryption))
	sb.WriteString("\n")

	return sb.String()
}

func (g *PostgreSQLConfigGenerator) generateResourceSection() string {
	var sb strings.Builder
	
	sb.WriteString(fmt.Sprintf(`# -----------------------------
# RESOURCE USAGE (MEMORY)
# -----------------------------
# Memory settings optimized for %.1f GB RAM

`, g.SystemInfo.TotalMemoryGB()))

	// Shared buffers with detailed description
	sb.WriteString(FormatConfigComment("postgres.shared_buffers", "#"))
	sb.WriteString("\n")
	sharedBuffers := types.Size(utils.KBToBytes(g.TunedParams.SharedBuffers))
	sb.WriteString(fmt.Sprintf("shared_buffers = %s\n", sharedBuffers.PostgreSQLMB()))
	sb.WriteString("\n")

	// Effective cache size with description
	sb.WriteString(FormatConfigComment("postgres.effective_cache_size", "#"))
	sb.WriteString("\n")
	effectiveCacheSize := types.Size(utils.KBToBytes(g.TunedParams.EffectiveCacheSize))
	sb.WriteString(fmt.Sprintf("effective_cache_size = %s\n", effectiveCacheSize.PostgreSQLMB()))
	sb.WriteString("\n")

	// Maintenance work mem with description
	sb.WriteString(FormatConfigComment("postgres.maintenance_work_mem", "#"))
	sb.WriteString("\n")
	maintenanceWorkMem := types.Size(utils.KBToBytes(g.TunedParams.MaintenanceWorkMem))
	sb.WriteString(fmt.Sprintf("maintenance_work_mem = %s\n", maintenanceWorkMem.PostgreSQLMB()))
	sb.WriteString("\n")

	// Work mem with description
	sb.WriteString(FormatConfigComment("postgres.work_mem", "#"))
	sb.WriteString("\n")
	workMem := types.Size(utils.KBToBytes(g.TunedParams.WorkMem))
	sb.WriteString(fmt.Sprintf("work_mem = %s\n", workMem.PostgreSQLMB()))
	sb.WriteString("\n")

	// Huge pages setting
	sb.WriteString("# Huge pages setting\n")
	sb.WriteString(fmt.Sprintf("huge_pages = %s", g.TunedParams.HugePages))
	if g.TunedParams.HugePages == "try" {
		sb.WriteString("  # Recommended for large memory systems")
	}
	sb.WriteString("\n\n")

	return sb.String()
}

func (g *PostgreSQLConfigGenerator) generateWALSection() string {
	sb := strings.Builder{}
	sb.WriteString(`# -----------------------------
# WRITE-AHEAD LOG
# -----------------------------

`)

	// WAL level with description
	sb.WriteString(FormatConfigComment("postgres.wal_level", "#"))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("wal_level = %s\n", g.TunedParams.WalLevel))
	sb.WriteString("\n")

	if g.TunedParams.MaxWalSenders != nil {
		sb.WriteString("# Maximum number of concurrent connections from standby servers\n")
		sb.WriteString("# Must be 0 when wal_level=minimal\n")
		sb.WriteString(fmt.Sprintf("max_wal_senders = %d\n", *g.TunedParams.MaxWalSenders))
		sb.WriteString("\n")
	}

	// WAL buffers with description
	sb.WriteString(FormatConfigComment("postgres.wal_buffers", "#"))
	sb.WriteString("\n")
	walBuffers := types.Size(utils.KBToBytes(g.TunedParams.WalBuffers))
	sb.WriteString(fmt.Sprintf("wal_buffers = %s\n", walBuffers.PostgreSQLMB()))
	sb.WriteString("\n")

	sb.WriteString("# Minimum WAL size\n")
	minWalSize := types.Size(utils.KBToBytes(g.TunedParams.MinWalSize))
	sb.WriteString(fmt.Sprintf("min_wal_size = %s\n", minWalSize.PostgreSQLMB()))
	sb.WriteString("\n")

	sb.WriteString("# Maximum WAL size\n")
	maxWalSize := types.Size(utils.KBToBytes(g.TunedParams.MaxWalSize))
	sb.WriteString(fmt.Sprintf("max_wal_size = %s\n", maxWalSize.PostgreSQLMB()))
	sb.WriteString("\n")

	// Checkpoint completion target with description
	sb.WriteString(FormatConfigComment("postgres.checkpoint_completion_target", "#"))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("checkpoint_completion_target = %.1f\n", g.TunedParams.CheckpointCompletionTarget))
	sb.WriteString("\n")

	return sb.String()
}

func (g *PostgreSQLConfigGenerator) generateQueryTuningSection() string {
	sb := strings.Builder{}
	sb.WriteString(`# -----------------------------
# QUERY TUNING
# -----------------------------

`)

	var diskComment string
	switch g.SystemInfo.DiskType {
	case sysinfo.DiskSSD:
		diskComment = " # Optimized for SSD"
	case sysinfo.DiskHDD:
		diskComment = " # Optimized for traditional HDD"
	case sysinfo.DiskSAN:
		diskComment = " # Optimized for SAN storage"
	}

	sb.WriteString(fmt.Sprintf("random_page_cost = %.1f%s\n", g.TunedParams.RandomPageCost, diskComment))

	if g.TunedParams.EffectiveIoConcurrency != nil {
		sb.WriteString(fmt.Sprintf("effective_io_concurrency = %d	# Concurrent I/O operations\n", *g.TunedParams.EffectiveIoConcurrency))
	} else {
		sb.WriteString("# effective_io_concurrency not available on this OS\n")
	}

	sb.WriteString(fmt.Sprintf("default_statistics_target = %d	# Statistics target\n\n", g.TunedParams.DefaultStatisticsTarget))

	return sb.String()
}

func (g *PostgreSQLConfigGenerator) generateParallelSection() string {
	sb := strings.Builder{}
	sb.WriteString(`# -----------------------------
# PARALLEL PROCESSING
# -----------------------------

`)

	if g.SystemInfo.CPUCount < 4 {
		sb.WriteString("# Parallel processing disabled (< 4 CPUs)\n")
		sb.WriteString("max_worker_processes = 8\n")
		sb.WriteString("max_parallel_workers_per_gather = 2\n")
		sb.WriteString("max_parallel_workers = 8\n\n")
	} else {
		sb.WriteString(fmt.Sprintf("# Parallel processing (optimized for %d CPUs)\n", g.SystemInfo.CPUCount))
		sb.WriteString(fmt.Sprintf("max_worker_processes = %d\n", g.TunedParams.MaxWorkerProcesses))
		sb.WriteString(fmt.Sprintf("max_parallel_workers_per_gather = %d\n", g.TunedParams.MaxParallelWorkersPerGather))
		sb.WriteString(fmt.Sprintf("max_parallel_workers = %d\n", g.TunedParams.MaxParallelWorkers))

		if g.TunedParams.MaxParallelMaintenanceWorkers != nil {
			sb.WriteString(fmt.Sprintf("max_parallel_maintenance_workers = %d	# PostgreSQL 11+\n", *g.TunedParams.MaxParallelMaintenanceWorkers))
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

func (g *PostgreSQLConfigGenerator) generateLoggingSection() string {
	logConnections := "off"
	// Note: LogConnections field not available in current schema
	logDisconnections := "off"
	// Note: LogDisconnections field not available in current schema

	return fmt.Sprintf(`# -----------------------------
# LOGGING
# -----------------------------

# Logging settings (adjust as needed for your environment)
log_statement = '%s'		# Log statement level
log_connections = %s		# Log connections
log_disconnections = %s		# Log disconnections

# Uncomment to enable query logging
# log_statement = 'all'		# Log all statements
# log_min_duration_statement = 1000	# Log slow queries (> 1 second)

`, g.config.LogStatement, logConnections, logDisconnections)
}

func (g *PostgreSQLConfigGenerator) generateSSLSection() string {
	sslEnabled := "off"
	// Note: Ssl field not available in current schema
	
	// Safely dereference pointer fields
	sslCertFile := ""
	if g.config.SslCertFile != nil {
		sslCertFile = *g.config.SslCertFile
	}
	
	sslKeyFile := ""
	if g.config.SslKeyFile != nil {
		sslKeyFile = *g.config.SslKeyFile
	}

	return fmt.Sprintf(`# -----------------------------
# SSL/TLS CONFIGURATION
# -----------------------------

# SSL settings
ssl = %s				# Enable SSL/TLS
ssl_cert_file = '%s'		# SSL certificate file
ssl_key_file = '%s'		# SSL private key file
# ssl_ca_file = 'root.crt'		# Certificate authority file
# ssl_min_protocol_version = 'TLSv1.2'	# Minimum TLS version
# ssl_ciphers = 'HIGH:MEDIUM:+3DES:!aNULL'	# Allowed ciphers

`, sslEnabled, sslCertFile, sslKeyFile)
}

func (g *PostgreSQLConfigGenerator) generateCustomSection() string {
	sb := strings.Builder{}
	sb.WriteString(`# -----------------------------
# CUSTOM SETTINGS
# -----------------------------

`)

	for key, value := range g.CustomSettings {
		sb.WriteString(fmt.Sprintf("%s = %s\n", key, value))
	}

	sb.WriteString("\n")
	return sb.String()
}

func (g *PostgreSQLConfigGenerator) generateWarningsSection() string {
	sb := strings.Builder{}
	sb.WriteString(`# -----------------------------
# WARNINGS
# -----------------------------

`)

	for _, warning := range g.TunedParams.Warnings {
		sb.WriteString(fmt.Sprintf("# %s\n", warning))
	}

	sb.WriteString("\n")
	return sb.String()
}

// AddCustomSetting adds a custom setting to the configuration
func (g *PostgreSQLConfigGenerator) AddCustomSetting(key, value string) {
	g.CustomSettings[key] = value
}

// inferDBType tries to infer the database type from tuned parameters
// This is a helper function since the generator doesn't store the original DB type
func inferDBType(params *pgtune.TunedParameters) string {
	// This is a simplified inference - in practice you might want to store the original DB type
	if params.MaxConnections <= 20 {
		return "desktop"
	}
	if params.MaxConnections >= 300 {
		return "oltp"
	}
	if params.MaxConnections <= 40 {
		return "dw"
	}
	if params.DefaultStatisticsTarget == 500 {
		return "dw"
	}
	return "web"
}

// getEffectiveIoConcurrency safely gets the effective_io_concurrency value, defaulting to 1 if nil
func getEffectiveIoConcurrency(value *int) int {
	if value == nil {
		return 1 // Default value
	}
	return *value
}

// generateExtensionSection generates the extension configuration section
func (g *PostgreSQLConfigGenerator) generateExtensionSection() string {
	var sb strings.Builder
	
	// Only generate section if extensions are configured or shared libraries are needed
	sharedLibs := g.generateSharedPreloadLibraries()
	includes := g.generateIncludeFiles()
	
	if sharedLibs == "" && includes == "" && len(g.extensions) == 0 {
		return ""
	}
	
	sb.WriteString(`# -----------------------------
# EXTENSIONS
# -----------------------------

`)

	// Shared preload libraries
	if sharedLibs != "" {
		sb.WriteString("# Shared libraries to preload at server startup\n")
		sb.WriteString(fmt.Sprintf("shared_preload_libraries = '%s'\n", sharedLibs))
		sb.WriteString("\n")
	}

	// Include files for extension configurations
	if includes != "" {
		sb.WriteString("# Include extension configuration files\n")
		sb.WriteString(fmt.Sprintf("include = '%s'\n", includes))
		sb.WriteString("\n")
	}

	// Extension-specific comments
	if len(g.extensions) > 0 {
		sb.WriteString("# Enabled extensions:\n")
		for name, ext := range g.extensions {
			if ext.Enabled {
				sb.WriteString(fmt.Sprintf("#   %s", name))
				if ext.Version != nil && *ext.Version != "" {
					sb.WriteString(fmt.Sprintf(" (version: %s)", *ext.Version))
				}
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// generateSharedPreloadLibraries generates the shared_preload_libraries setting based on enabled extensions
func (g *PostgreSQLConfigGenerator) generateSharedPreloadLibraries() string {
	var libraries []string
	
	// Check enabled extensions that need to be preloaded
	for _, ext := range g.extensions {
		if ext.Enabled {
			switch ext.Name {
			case "pgaudit":
				libraries = append(libraries, "pgaudit")
			case "pg_stat_statements":
				libraries = append(libraries, "pg_stat_statements")
			case "auto_explain":
				libraries = append(libraries, "auto_explain")
			case "pg_cron":
				libraries = append(libraries, "pg_cron")
			case "timescaledb":
				libraries = append(libraries, "timescaledb")
			}
		}
	}
	
	if len(libraries) == 0 {
		return ""
	}
	
	return strings.Join(libraries, ",")
}

// generateIncludeFiles generates the include directive for extension config files
func (g *PostgreSQLConfigGenerator) generateIncludeFiles() string {
	var includes []string
	
	// Check for PGAudit configuration
	if g.pgauditConf != nil {
		pgauditGen := NewPGAuditConfigGenerator(g.pgauditConf)
		if pgauditGen.IsEnabled() {
			includes = append(includes, "postgres.pgaudit.conf")
		}
	}
	
	// Check for other extensions that need config files
	for _, ext := range g.extensions {
		if ext.Enabled && ext.ConfigFile != nil && *ext.ConfigFile != "" {
			includes = append(includes, *ext.ConfigFile)
		}
	}
	
	if len(includes) == 0 {
		return ""
	}
	
	return strings.Join(includes, ",")
}
