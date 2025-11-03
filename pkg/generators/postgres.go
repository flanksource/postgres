package generators

import (
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/postgres/pkg"
	"github.com/flanksource/postgres/pkg/pgtune"
	"github.com/flanksource/postgres/pkg/sysinfo"
)

// PostgreSQLConfigGenerator generates postgresql.conf configuration
type PostgreSQLConfigGenerator struct {
	SystemInfo     *sysinfo.SystemInfo
	TunedParams    *pgtune.TunedParameters
	CustomSettings map[string]string
	config         *pkg.PostgresConf              // Store generated config for template use
	extensions     map[string]pkg.ExtensionConfig // Store extensions
	pgauditConf    *pkg.PGAuditConf               // Store PGAudit config
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
	// Helper functions removed - no longer needed for non-pointer types

	config := &pkg.PostgresConf{
		// Connection settings (using actual field names from schema)
		Port:               5432,
		MaxConnections:     g.TunedParams.MaxConnections,
		PasswordEncryption: "md5",

		// SSL settings
		SSLCertFile: "/etc/ssl/certs/server.crt",
		SSLKeyFile:  "/etc/ssl/private/server.key",

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

	tuned, err := g.TunedParams.AsConf()
	if err != nil {
		panic(fmt.Sprintf("failed to convert tuned parameters to map: %v", err))
	}
	sb.WriteString(tuned.AsFile() + "\n")

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
		g.SystemInfo.EffectiveCPUCount(),
		g.SystemInfo.PostgreSQLVersion,
		g.TunedParams.MaxConnections,

		g.SystemInfo.DiskType,
	)
}

func FormatConfigComment(key, prefix string) string {

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("%s %s\n", prefix, key))
	return sb.String()
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

	if len(includes) == 0 {
		return ""
	}

	return strings.Join(includes, ",")
}
