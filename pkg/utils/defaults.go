package utils

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/flanksource/postgres/pkg/schemas"
)

// ResolveDefault resolves environment variable syntax in default values
// Supports patterns like:
// - ${VAR} - returns environment variable VAR or empty string
// - ${VAR:-default} - returns environment variable VAR or "default"
// - plain text - returns as-is
func ResolveDefault(value string) string {
	if value == "" {
		return value
	}

	// Pattern to match ${VAR} or ${VAR:-default}
	re := regexp.MustCompile(`\$\{([A-Z_][A-Z0-9_]*)(:-([^}]+))?\}`)

	return re.ReplaceAllStringFunc(value, func(match string) string {
		parts := re.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match // Return original if no match
		}

		envVar := parts[1]
		defaultVal := ""

		if len(parts) >= 4 && parts[3] != "" {
			defaultVal = parts[3]
		}

		// Get environment variable
		if envValue := os.Getenv(envVar); envValue != "" {
			return envValue
		}

		return defaultVal
	})
}

// ResolveBoolDefault resolves a boolean default value with environment variable support
func ResolveBoolDefault(value string, fallback bool) bool {
	resolved := ResolveDefault(value)
	if resolved == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(resolved)
	if err != nil {
		return fallback
	}

	return parsed
}

// ResolveIntDefault resolves an integer default value with environment variable support
func ResolveIntDefault(value string, fallback int) int {
	resolved := ResolveDefault(value)
	if resolved == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(resolved)
	if err != nil {
		return fallback
	}

	return parsed
}

// ResolveFloat64Default resolves a float64 default value with environment variable support
func ResolveFloat64Default(value string, fallback float64) float64 {
	resolved := ResolveDefault(value)
	if resolved == "" {
		return fallback
	}

	parsed, err := strconv.ParseFloat(resolved, 64)
	if err != nil {
		return fallback
	}

	return parsed
}

// GenerateSchemaDefaultsFromDescribeConfig creates schema defaults from postgres --describe-config output
// This function bridges the gap between PostgreSQL's native parameter metadata and our configuration schema
func GenerateSchemaDefaultsFromDescribeConfig(params []schemas.Param) map[string]string {
	defaults := make(map[string]string)

	for _, param := range params {
		if param.BootVal == "" {
			continue
		}

		// Convert PostgreSQL parameter names to schema keys
		schemaKey := fmt.Sprintf("postgres.%s", param.Name)

		// Create environment variable pattern for the default
		envVar := fmt.Sprintf("POSTGRES_%s", strings.ToUpper(strings.ReplaceAll(param.Name, ".", "_")))
		envDefault := fmt.Sprintf("${%s:-%s}", envVar, param.BootVal)

		defaults[schemaKey] = envDefault
	}

	return defaults
}

// MergeSchemaDefaults merges manually curated defaults with auto-generated ones
// Manual defaults take precedence over auto-generated ones
func MergeSchemaDefaults(manualDefaults, autoDefaults map[string]string) map[string]string {
	merged := make(map[string]string)

	// Start with auto-generated defaults
	for key, value := range autoDefaults {
		merged[key] = value
	}

	// Override with manual defaults
	for key, value := range manualDefaults {
		merged[key] = value
	}

	return merged
}

// GetSchemaDefaultsFromPostgres generates schema defaults using embedded PostgreSQL's describe-config
// This function provides the most accurate and up-to-date defaults based on the actual PostgreSQL version
func GetSchemaDefaultsFromPostgres(version string) (map[string]string, error) {
	// Note: This requires importing pkg which would create a circular dependency
	// In practice, this would be called from a higher-level package
	// For now, we'll return an error directing users to the correct approach
	return nil, fmt.Errorf("use NewEmbeddedPostgres().DescribeConfig() from a higher-level package to avoid import cycles")
}

// GetSchemaDefaults extracts default values from JSON schema metadata
// This would be used by configuration loaders to set proper defaults
func GetSchemaDefaults() map[string]string {
	// This map would normally be generated from the JSON schema or postgres --describe-config
	// For now, we'll implement it as a static map based on our schema
	return map[string]string{
		"postgres.port":                                "${POSTGRES_PORT:-5432}",
		"postgres.max_connections":                     "${POSTGRES_MAX_CONNECTIONS:-100}",
		"postgres.shared_buffers":                      "${POSTGRES_SHARED_BUFFERS:-128MB}",
		"postgres.effective_cache_size":                "${POSTGRES_EFFECTIVE_CACHE_SIZE:-4GB}",
		"postgres.maintenance_work_mem":                "${POSTGRES_MAINTENANCE_WORK_MEM:-64MB}",
		"postgres.checkpoint_completion_target":        "${POSTGRES_CHECKPOINT_COMPLETION_TARGET:-0.9}",
		"postgres.wal_buffers":                         "${POSTGRES_WAL_BUFFERS:-16MB}",
		"postgres.work_mem":                            "${POSTGRES_WORK_MEM:-4MB}",
		"postgres.random_page_cost":                    "${POSTGRES_RANDOM_PAGE_COST:-4.0}",
		"postgres.effective_io_concurrency":            "${POSTGRES_EFFECTIVE_IO_CONCURRENCY:-1}",
		"postgres.listen_addresses":                    "${POSTGRES_LISTEN_ADDRESSES:-localhost}",
		"postgres.log_connections":                     "${POSTGRES_LOG_CONNECTIONS:-false}",
		"postgres.log_disconnections":                  "${POSTGRES_LOG_DISCONNECTIONS:-false}",
		"postgres.log_statement":                       "${POSTGRES_LOG_STATEMENT:-none}",
		"postgres.password_encryption":                 "${POSTGRES_PASSWORD_ENCRYPTION:-scram-sha-256}",
		"postgres.ssl":                                 "${POSTGRES_SSL:-false}",
		"postgres.ssl_cert_file":                       "${POSTGRES_SSL_CERT_FILE:-server.crt}",
		"postgres.ssl_key_file":                        "${POSTGRES_SSL_KEY_FILE:-server.key}",
		"postgres.ssl_min_protocol_version":            "${POSTGRES_SSL_MIN_PROTOCOL_VERSION:-TLSv1.2}",
		"postgres.ssl_ciphers":                         "${POSTGRES_SSL_CIPHERS:-HIGH:MEDIUM:+3DES:!aNULL}",
		"postgres.wal_level":                           "${POSTGRES_WAL_LEVEL:-replica}",
		"postgres.statement_timeout":                   "${POSTGRES_STATEMENT_TIMEOUT:-0}",
		"postgres.lock_timeout":                        "${POSTGRES_LOCK_TIMEOUT:-0}",
		"postgres.idle_in_transaction_session_timeout": "${POSTGRES_IDLE_IN_TRANSACTION_SESSION_TIMEOUT:-0}",

		// PgBouncer defaults
		"pgbouncer.listen_address": "${PGBOUNCER_LISTEN_ADDRESS:-127.0.0.1}",
		"pgbouncer.listen_port":    "${PGBOUNCER_LISTEN_PORT:-6432}",
		"pgbouncer.admin_user":     "${PGBOUNCER_ADMIN_USER:-postgres}",

		// PostgREST defaults
		"postgrest.server_host":     "${SERVER_HOST:-!4}",
		"postgrest.server_port":     "${SERVER_PORT:-3000}",
		"postgrest.db_pool":         "${DB_POOL:-10}",
		"postgrest.db_pool_timeout": "${DB_POOL_TIMEOUT:-10}",
		"postgrest.db_schemas":      "${DB_SCHEMAS:-public}",
		"postgrest.db_uri":          "${DB_URI:-postgresql://postgres@localhost:5432/postgres}",
		"postgrest.admin_role":      "${ADMIN_ROLE:-postgres}",
		"postgrest.anonymous_role":  "${ANONYMOUS_ROLE:-postgres}",
		"postgrest.log_level":       "${LOG_LEVEL:-error}",

		// WAL-G defaults
		"walg.enabled":                "${WALG_ENABLED:-false}",
		"walg.backup_compress":        "${WALG_BACKUP_COMPRESS:-lz4}",
		"walg.backup_retain_count":    "${WALG_BACKUP_RETAIN_COUNT:-7}",
		"walg.backup_schedule":        "${WALG_BACKUP_SCHEDULE:-0 2 * * *}",
		"walg.postgresql_host":        "${WALG_POSTGRESQL_HOST:-localhost}",
		"walg.postgresql_port":        "${WALG_POSTGRESQL_PORT:-5432}",
		"walg.postgresql_user":        "${WALG_POSTGRESQL_USER:-postgres}",
		"walg.postgresql_database":    "${WALG_POSTGRESQL_DATABASE:-postgres}",
		"walg.initial_backup":         "${WALG_INITIAL_BACKUP:-false}",
		"walg.s3_use_ssl":             "${WALG_S3_SSL:-true}",
		"walg.stream_create_command":  "${WALG_STREAM_CREATE_COMMAND:-pg_receivewal -h localhost -p 5432 -U postgres -D - --synchronous}",
		"walg.stream_restore_command": "${WALG_STREAM_RESTORE_COMMAND:-pg_receivewal -h localhost -p 5432 -U postgres -D - --synchronous}",

		// PGAudit defaults
		"pgaudit.log":                    "${PGAUDIT_LOG:-none}",
		"pgaudit.log_catalog":            "${PGAUDIT_LOG_CATALOG:-on}",
		"pgaudit.log_client":             "${PGAUDIT_LOG_CLIENT:-off}",
		"pgaudit.log_level":              "${PGAUDIT_LOG_LEVEL:-log}",
		"pgaudit.log_parameter":          "${PGAUDIT_LOG_PARAMETER:-off}",
		"pgaudit.log_parameter_max_size": "${PGAUDIT_LOG_PARAMETER_MAX_SIZE:-0}",
		"pgaudit.log_relation":           "${PGAUDIT_LOG_RELATION:-off}",
		"pgaudit.log_rows":               "${PGAUDIT_LOG_ROWS:-off}",
		"pgaudit.log_statement":          "${PGAUDIT_LOG_STATEMENT:-on}",
		"pgaudit.log_statement_once":     "${PGAUDIT_LOG_STATEMENT_ONCE:-off}",
	}
}
