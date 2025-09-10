package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// SchemaProperty represents a JSON schema property
type SchemaProperty struct {
	Type        string                 `json:"type,omitempty"`
	Description string                 `json:"description,omitempty"`
	Default     interface{}            `json:"default,omitempty"`
	Enum        []interface{}          `json:"enum,omitempty"`
	Pattern     string                 `json:"pattern,omitempty"`
	Minimum     *float64               `json:"minimum,omitempty"`
	Maximum     *float64               `json:"maximum,omitempty"`
	XType       string                 `json:"x-type,omitempty"`
	XSensitive  bool                   `json:"x-sensitive,omitempty"`
	Properties  map[string]*SchemaProperty `json:"properties,omitempty"`
	Items       *SchemaProperty        `json:"items,omitempty"`
}

// Param represents a PostgreSQL parameter from describe-config
type Param struct {
	Name            string
	Setting         string
	Unit            string
	Category        string
	ShortDesc       string
	ExtraDesc       string
	Context         string
	Vartype         string
	Source          string
	MinVal          string
	MaxVal          string
	EnumVals        []string
	BootVal         string
	ResetVal        string
	SourceFile      string
	SourceLine      string
	PendingRestart  bool
}

// parseDescribeConfigOutput parses the describe-config output string into parameters
func parseDescribeConfigOutput(output string) ([]Param, error) {
	var params []Param
	
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("invalid describe-config output: missing header or data")
	}
	
	// Skip the header line
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		
		// Parse the pipe-separated values
		parts := strings.Split(line, "|")
		if len(parts) < 16 {
			continue // Skip incomplete lines
		}
		
		// Parse enum values if present
		var enumVals []string
		if strings.TrimSpace(parts[10]) != "" {
			enumStr := strings.TrimSpace(parts[10])
			if enumStr != "" && enumStr != "\\N" {
				// Parse enum values like {val1,val2,val3}
				enumStr = strings.Trim(enumStr, "{}")
				if enumStr != "" {
					enumVals = strings.Split(enumStr, ",")
					for j, val := range enumVals {
						enumVals[j] = strings.TrimSpace(val)
					}
				}
			}
		}
		
		param := Param{
			Name:            strings.TrimSpace(parts[0]),
			Setting:         strings.TrimSpace(parts[1]),
			Unit:            strings.TrimSpace(parts[2]),
			Category:        strings.TrimSpace(parts[3]),
			ShortDesc:       strings.TrimSpace(parts[4]),
			ExtraDesc:       strings.TrimSpace(parts[5]),
			Context:         strings.TrimSpace(parts[6]),
			Vartype:         strings.TrimSpace(parts[7]),
			Source:          strings.TrimSpace(parts[8]),
			MinVal:          strings.TrimSpace(parts[9]),
			MaxVal:          strings.TrimSpace(parts[10]),
			EnumVals:        enumVals,
			BootVal:         strings.TrimSpace(parts[11]),
			ResetVal:        strings.TrimSpace(parts[12]),
			SourceFile:      strings.TrimSpace(parts[13]),
			SourceLine:      strings.TrimSpace(parts[14]),
			PendingRestart:  strings.TrimSpace(parts[15]) == "t",
		}
		
		params = append(params, param)
	}
	
	return params, nil
}

// convertParamToProperty converts a PostgreSQL parameter to a JSON schema property
func convertParamToProperty(param Param) *SchemaProperty {
	// Combine short description with extra documentation
	description := param.ShortDesc
	if param.ExtraDesc != "" && param.ExtraDesc != "\\N" {
		if description != "" {
			description += " " + param.ExtraDesc
		} else {
			description = param.ExtraDesc
		}
	}
	
	prop := &SchemaProperty{
		Description: description,
	}
	
	// Handle different parameter types
	switch param.Vartype {
	case "bool":
		prop.Type = "boolean"
		if param.BootVal != "" && param.BootVal != "\\N" {
			if param.BootVal == "on" || param.BootVal == "true" {
				prop.Default = true
			} else {
				prop.Default = false
			}
		}
		
	case "integer":
		prop.Type = "integer"
		if param.BootVal != "" && param.BootVal != "\\N" {
			if val, err := strconv.Atoi(param.BootVal); err == nil {
				prop.Default = val
			}
		}
		if param.MinVal != "" && param.MinVal != "\\N" {
			if val, err := strconv.ParseFloat(param.MinVal, 64); err == nil {
				prop.Minimum = &val
			}
		}
		if param.MaxVal != "" && param.MaxVal != "\\N" {
			if val, err := strconv.ParseFloat(param.MaxVal, 64); err == nil {
				prop.Maximum = &val
			}
		}
		
	case "real":
		prop.Type = "number"
		if param.BootVal != "" && param.BootVal != "\\N" {
			if val, err := strconv.ParseFloat(param.BootVal, 64); err == nil {
				prop.Default = val
			}
		}
		if param.MinVal != "" && param.MinVal != "\\N" {
			if val, err := strconv.ParseFloat(param.MinVal, 64); err == nil {
				prop.Minimum = &val
			}
		}
		if param.MaxVal != "" && param.MaxVal != "\\N" {
			if val, err := strconv.ParseFloat(param.MaxVal, 64); err == nil {
				prop.Maximum = &val
			}
		}
		
	case "string":
		prop.Type = "string"
		if param.BootVal != "" && param.BootVal != "\\N" {
			prop.Default = param.BootVal
		}
		
		// Handle enum values
		if len(param.EnumVals) > 0 {
			for _, val := range param.EnumVals {
				prop.Enum = append(prop.Enum, val)
			}
		}
		
	case "enum":
		prop.Type = "string"
		if param.BootVal != "" && param.BootVal != "\\N" {
			prop.Default = param.BootVal
		}
		if len(param.EnumVals) > 0 {
			for _, val := range param.EnumVals {
				prop.Enum = append(prop.Enum, val)
			}
		}
		
	default:
		prop.Type = "string"
		if param.BootVal != "" && param.BootVal != "\\N" {
			prop.Default = param.BootVal
		}
	}
	
	// Handle special types based on parameter name or unit
	if isMemoryParam(param.Name) || param.Unit == "kB" || param.Unit == "8kB" {
		prop.XType = "Size"
		prop.Pattern = "^[0-9]+[kMGT]?B?$"
	}
	
	if isTimeParam(param.Name) || param.Unit == "ms" || param.Unit == "s" || param.Unit == "min" {
		prop.XType = "Duration"
	}
	
	if isPasswordParam(param.Name) {
		prop.XSensitive = true
	}
	
	return prop
}

// isMemoryParam checks if a parameter is memory-related
func isMemoryParam(name string) bool {
	memoryParams := []string{
		"shared_buffers", "work_mem", "maintenance_work_mem", "effective_cache_size",
		"wal_buffers", "temp_buffers", "max_stack_depth", "dynamic_shared_memory_main_size",
	}
	for _, param := range memoryParams {
		if param == name {
			return true
		}
	}
	return false
}

// isTimeParam checks if a parameter is time-related
func isTimeParam(name string) bool {
	timeParams := []string{
		"statement_timeout", "lock_timeout", "idle_in_transaction_session_timeout",
		"checkpoint_timeout", "wal_receiver_timeout", "wal_sender_timeout",
	}
	for _, param := range timeParams {
		if param == name {
			return true
		}
	}
	return false
}

// isPasswordParam checks if a parameter is password-related
func isPasswordParam(name string) bool {
	return strings.Contains(strings.ToLower(name), "password")
}

// generatePostgresSchema generates the PostgreSQL schema from describe-config output
func generatePostgresSchema(describeConfigOutput string) (map[string]*SchemaProperty, error) {
	params, err := parseDescribeConfigOutput(describeConfigOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to parse describe-config output: %w", err)
	}
	
	properties := make(map[string]*SchemaProperty)
	
	for _, param := range params {
		prop := convertParamToProperty(param)
		if prop != nil {
			properties[param.Name] = prop
		}
	}
	
	return properties, nil
}

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s <describe-config-output>\n", os.Args[0])
		fmt.Println("The describe-config-output should be the raw output from PostgreSQL's describe-config command")
		os.Exit(1)
	}
	
	describeConfigOutput := os.Args[1]
	
	fmt.Println("Generating JSON schema from PostgreSQL describe-config...")
	
	// Generate PostgreSQL configuration schema from the provided output
	postgresSchema, err := generatePostgresSchema(describeConfigOutput)
	if err != nil {
		fmt.Printf("Error generating PostgreSQL schema: %v\n", err)
		os.Exit(1)
	}
	
	// Get hardcoded schemas for other services
	pgbouncerSchema := getPgBouncerSchema()
	databaseConfigSchema := getDatabaseConfigSchema()
	postgrestSchema := getPostgRESTSchema()
	walgSchema := getWalGSchema()
	pgauditSchema := getPGAuditSchema()
	pghbaSchema := getPgHBASchema()
	
	// Create the full schema structure
	schema := map[string]interface{}{
		"$id":                  "https://github.com/flanksource/postgres/schema/pgconfig.json",
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"type":                 "object",
		"additionalProperties": false,
		"definitions": map[string]interface{}{
			"PostgresConf": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"description":          "Main PostgreSQL server configuration",
				"properties":           postgresSchema,
			},
			"PgBouncerConf":   pgbouncerSchema,
			"DatabaseConfig":  databaseConfigSchema,
			"PostgrestConf":   postgrestSchema,
			"WalgConf":        walgSchema,
			"PGAuditConf":     pgauditSchema,
			"PgHBAConf":       pghbaSchema,
		},
		"properties": map[string]interface{}{
			"postgres": map[string]interface{}{
				"$ref": "#/definitions/PostgresConf",
			},
			"pgbouncer": map[string]interface{}{
				"$ref": "#/definitions/PgBouncerConf",
			},
			"postgrest": map[string]interface{}{
				"$ref": "#/definitions/PostgrestConf",
			},
			"walg": map[string]interface{}{
				"$ref": "#/definitions/WalgConf",
			},
			"pgaudit": map[string]interface{}{
				"$ref": "#/definitions/PGAuditConf",
			},
			"pghba": map[string]interface{}{
				"$ref": "#/definitions/PgHBAConf",
			},
		},
	}
	
	// Write schema to file
	schemaBytes, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling schema: %v\n", err)
		os.Exit(1)
	}
	
	schemaPath := "schema/pgconfig-schema.json"
	if err := os.WriteFile(schemaPath, schemaBytes, 0644); err != nil {
		fmt.Printf("Error writing schema to %s: %v\n", schemaPath, err)
		os.Exit(1)
	}
	
	// Also write individual component schemas
	writeComponentSchemas(map[string]interface{}{
		"pgbouncer": pgbouncerSchema,
		"postgrest": postgrestSchema,
		"walg":      walgSchema,
		"pgaudit":   pgauditSchema,
		"pghba":     pghbaSchema,
	})
	
	fmt.Printf("✅ Successfully generated schema: %s\n", schemaPath)
	fmt.Printf("   PostgreSQL properties: %d\n", len(postgresSchema))
	fmt.Printf("   Service definitions: PostgresConf, PgBouncerConf, PostgrestConf, WalgConf, PGAuditConf, PgHBAConf, DatabaseConfig\n")
}

// writeComponentSchemas writes individual component schemas to separate files
func writeComponentSchemas(schemas map[string]interface{}) {
	for name, schema := range schemas {
		schemaBytes, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			fmt.Printf("Warning: Error marshaling %s schema: %v\n", name, err)
			continue
		}
		
		filePath := fmt.Sprintf("schema/%s-schema.json", name)
		if err := os.WriteFile(filePath, schemaBytes, 0644); err != nil {
			fmt.Printf("Warning: Error writing %s schema to %s: %v\n", name, filePath, err)
		} else {
			fmt.Printf("✅ Generated %s schema: %s\n", name, filePath)
		}
	}
}

// getPgBouncerSchema returns the JSON schema definition for PgBouncer configuration
func getPgBouncerSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"description":          "PgBouncer connection pooler configuration",
		"properties": map[string]interface{}{
			"admin_password": map[string]interface{}{
				"type":        "string",
				"description": "Administrative password for PgBouncer",
				"x-sensitive": true,
			},
			"admin_user": map[string]interface{}{
				"type":        "string",
				"description": "Administrative user for PgBouncer",
			},
			"auth_type": map[string]interface{}{
				"type":        "string",
				"description": "Authentication type for PgBouncer",
				"enum":        []string{"any", "trust", "plain", "md5", "scram-sha-256", "cert", "hba", "pam"},
				"default":     "md5",
			},
			"default_pool_size": map[string]interface{}{
				"type":        "integer",
				"description": "Default pool size for databases",
				"minimum":     1,
				"default":     25,
			},
			"listen_address": map[string]interface{}{
				"type":        "string",
				"description": "Specifies the address to listen on",
				"default":     "0.0.0.0",
			},
			"listen_port": map[string]interface{}{
				"type":        "integer",
				"description": "Specifies the port to listen on",
				"minimum":     1,
				"maximum":     65535,
				"default":     6432,
			},
			"max_client_conn": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of client connections allowed",
				"minimum":     1,
				"default":     100,
			},
			"pool_mode": map[string]interface{}{
				"type":        "string",
				"description": "Pooling mode to use",
				"enum":        []string{"session", "transaction", "statement"},
				"default":     "transaction",
			},
			"server_reset_query": map[string]interface{}{
				"type":        "string",
				"description": "Query to run on server connection before returning to pool",
				"default":     "DISCARD ALL",
			},
			"auth_file": map[string]interface{}{
				"type":        "string",
				"description": "Path to authentication file",
				"default":     "userlist.txt",
			},
			"auth_query": map[string]interface{}{
				"type":        "string",
				"description": "Query to authenticate users",
				"default":     "SELECT usename, passwd FROM pg_shadow WHERE usename=$1",
			},
			"min_pool_size": map[string]interface{}{
				"type":        "integer",
				"description": "Minimum pool size",
				"minimum":     0,
				"default":     0,
			},
			"reserve_pool_size": map[string]interface{}{
				"type":        "integer",
				"description": "Reserved pool size",
				"minimum":     0,
			},
			"server_lifetime": map[string]interface{}{
				"type":        "string",
				"description": "Maximum lifetime of a server connection",
				"pattern":     "^[0-9]+(us|ms|s|min|h|d)?$",
				"default":     "3600s",
			},
			"server_idle_timeout": map[string]interface{}{
				"type":        "string",
				"description": "Maximum idle time for server connections",
				"pattern":     "^[0-9]+(us|ms|s|min|h|d)?$",
				"default":     "600s",
			},
			"query_timeout": map[string]interface{}{
				"type":        "string",
				"description": "Query timeout",
				"pattern":     "^[0-9]+(us|ms|s|min|h|d)?$",
				"default":     "0",
			},
			"client_idle_timeout": map[string]interface{}{
				"type":        "string",
				"description": "Maximum idle time for client connections",
				"pattern":     "^[0-9]+(us|ms|s|min|h|d)?$",
				"default":     "0",
			},
			"databases": map[string]interface{}{
				"type":        "object",
				"description": "Database connection configurations",
				"additionalProperties": map[string]interface{}{
					"$ref": "#/definitions/DatabaseConfig",
				},
			},
		},
	}
}

// getDatabaseConfigSchema returns the JSON schema for database configuration
func getDatabaseConfigSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"description":          "Database connection configuration for PgBouncer",
		"properties": map[string]interface{}{
			"dbname": map[string]interface{}{
				"type":        "string",
				"description": "Database name",
			},
			"host": map[string]interface{}{
				"type":        "string",
				"description": "Database host",
				"default":     "localhost",
			},
			"port": map[string]interface{}{
				"type":        "integer",
				"description": "Database port",
				"minimum":     1,
				"maximum":     65535,
				"default":     5432,
			},
			"user": map[string]interface{}{
				"type":        "string",
				"description": "Database user",
			},
			"password": map[string]interface{}{
				"type":        "string",
				"description": "Database password",
				"x-sensitive": true,
			},
			"pool_size": map[string]interface{}{
				"type":        "integer",
				"description": "Pool size for this database",
				"minimum":     1,
			},
			"connect_query": map[string]interface{}{
				"type":        "string",
				"description": "Query to run on new connections",
			},
		},
	}
}

// getPostgRESTSchema returns the JSON schema definition for PostgREST configuration
func getPostgRESTSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"description":          "PostgREST API server configuration",
		"properties": map[string]interface{}{
			"db_uri": map[string]interface{}{
				"type":        "string",
				"description": "Database connection URI",
				"pattern":     "^postgres(ql)?://.*",
			},
			"db_schemas": map[string]interface{}{
				"type":        "string",
				"description": "Database schemas to expose via API",
				"default":     "public",
			},
			"db_pool": map[string]interface{}{
				"type":        "integer",
				"description": "Database connection pool size",
				"minimum":     1,
				"maximum":     1000,
				"default":     10,
			},
			"db_pool_timeout": map[string]interface{}{
				"type":        "integer",
				"description": "Database connection pool timeout in seconds",
				"minimum":     1,
				"default":     10,
			},
			"jwt_secret": map[string]interface{}{
				"type":        "string",
				"description": "JWT secret for authentication",
				"x-sensitive": true,
			},
			"jwt_aud": map[string]interface{}{
				"type":        "string",
				"description": "JWT audience claim",
				"default":     "",
			},
			"admin_role": map[string]interface{}{
				"type":        "string",
				"description": "Database role with admin privileges",
				"default":     "postgres",
			},
			"anonymous_role": map[string]interface{}{
				"type":        "string",
				"description": "Database role for anonymous access",
				"default":     "anon",
			},
			"server_host": map[string]interface{}{
				"type":        "string",
				"description": "Server host address",
				"default":     "0.0.0.0",
			},
			"server_port": map[string]interface{}{
				"type":        "integer",
				"description": "Server port",
				"minimum":     1,
				"maximum":     65535,
				"default":     3000,
			},
			"log_level": map[string]interface{}{
				"type":        "string",
				"description": "Logging level",
				"enum":        []string{"crit", "error", "warn", "info", "debug"},
				"default":     "error",
			},
			"pre_request": map[string]interface{}{
				"type":        "string",
				"description": "Pre-request function to call",
				"default":     "",
			},
			"server_ssl_cert": map[string]interface{}{
				"type":        "string",
				"description": "Path to SSL certificate file",
				"default":     "",
			},
			"server_ssl_key": map[string]interface{}{
				"type":        "string",
				"description": "Path to SSL private key file",
				"default":     "",
			},
			"max_rows": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum rows returned in a single response",
				"minimum":     1,
			},
			"role_claim_key": map[string]interface{}{
				"type":        "string",
				"description": "JWT claim key for role",
				"default":     "role",
			},
			"jwt_secret_is_base64": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether JWT secret is base64 encoded",
				"default":     false,
			},
		},
	}
}

// getWalGSchema returns the JSON schema definition for WAL-G configuration
func getWalGSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"description":          "WAL-G backup and archiving configuration",
		"properties": map[string]interface{}{
			"enabled": map[string]interface{}{
				"type":        "boolean",
				"description": "Enable or disable WAL-G",
				"default":     false,
			},
			"postgresql_data_dir": map[string]interface{}{
				"type":        "string",
				"description": "PostgreSQL data directory path",
				"default":     "/var/lib/postgresql/data",
			},
			"s3_prefix": map[string]interface{}{
				"type":        "string",
				"description": "S3 storage prefix (e.g., s3://bucket/path/to/folder)",
				"pattern":     "^s3://.*",
			},
			"gs_prefix": map[string]interface{}{
				"type":        "string",
				"description": "Google Cloud Storage prefix (e.g., gs://bucket/path/to/folder)",
				"pattern":     "^gs://.*",
			},
			"az_prefix": map[string]interface{}{
				"type":        "string",
				"description": "Azure Storage prefix",
			},
			"file_prefix": map[string]interface{}{
				"type":        "string",
				"description": "Local file system prefix for backups",
			},
			"s3_region": map[string]interface{}{
				"type":        "string",
				"description": "AWS S3 region",
				"default":     "us-east-1",
			},
			"s3_access_key": map[string]interface{}{
				"type":        "string",
				"description": "AWS S3 access key ID",
				"x-sensitive": true,
			},
			"s3_secret_key": map[string]interface{}{
				"type":        "string",
				"description": "AWS S3 secret access key",
				"x-sensitive": true,
			},
			"s3_session_token": map[string]interface{}{
				"type":        "string",
				"description": "AWS S3 session token (for temporary credentials)",
				"x-sensitive": true,
			},
			"s3_endpoint": map[string]interface{}{
				"type":        "string",
				"description": "Custom S3 endpoint URL",
			},
			"s3_use_ssl": map[string]interface{}{
				"type":        "boolean",
				"description": "Use SSL for S3 connections",
				"default":     true,
			},
			"gs_service_account_key": map[string]interface{}{
				"type":        "string",
				"description": "Google Cloud service account key JSON",
				"x-sensitive": true,
			},
			"gs_project_id": map[string]interface{}{
				"type":        "string",
				"description": "Google Cloud project ID",
			},
			"az_account_name": map[string]interface{}{
				"type":        "string",
				"description": "Azure storage account name",
			},
			"az_account_key": map[string]interface{}{
				"type":        "string",
				"description": "Azure storage account key",
				"x-sensitive": true,
			},
			"backup_schedule": map[string]interface{}{
				"type":        "string",
				"description": "Backup schedule in cron format",
				"default":     "0 2 * * *",
			},
			"backup_retain_count": map[string]interface{}{
				"type":        "integer",
				"description": "Number of backups to retain",
				"minimum":     1,
				"default":     7,
			},
			"stream_create_command": map[string]interface{}{
				"type":        "string",
				"description": "Command to create WAL stream",
			},
			"stream_restore_command": map[string]interface{}{
				"type":        "string",
				"description": "Command to restore from WAL stream",
			},
			"compression_method": map[string]interface{}{
				"type":        "string",
				"description": "Compression method for backups",
				"enum":        []string{"lz4", "lzma", "brotli", "zstd"},
				"default":     "lz4",
			},
			"compression_level": map[string]interface{}{
				"type":        "integer",
				"description": "Compression level (0-9)",
				"minimum":     0,
				"maximum":     9,
				"default":     1,
			},
			"wal_verify_checksum": map[string]interface{}{
				"type":        "boolean",
				"description": "Verify WAL checksums during backup",
				"default":     true,
			},
			"delta_max_steps": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum steps for delta backups",
				"minimum":     1,
				"default":     32,
			},
			"upload_concurrency": map[string]interface{}{
				"type":        "integer",
				"description": "Number of concurrent uploads",
				"minimum":     1,
				"maximum":     100,
				"default":     16,
			},
			"upload_disk_concurrency": map[string]interface{}{
				"type":        "integer",
				"description": "Number of concurrent disk operations",
				"minimum":     1,
				"maximum":     100,
				"default":     1,
			},
		},
	}
}

// getPGAuditSchema returns the JSON schema definition for PGAudit configuration
func getPGAuditSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"description":          "PGAudit extension configuration for PostgreSQL audit logging",
		"properties": map[string]interface{}{
			"log": map[string]interface{}{
				"type":        "string",
				"description": "Specifies which classes of statements will be logged by session audit logging",
				"enum":        []string{"none", "read", "write", "function", "role", "ddl", "misc", "misc_set", "all"},
				"default":     "none",
			},
			"log_catalog": map[string]interface{}{
				"type":        "string",
				"description": "Specifies that session logging should be enabled in the case where all relations in a statement are in pg_catalog",
				"enum":        []string{"on", "off"},
				"default":     "on",
			},
			"log_client": map[string]interface{}{
				"type":        "string",
				"description": "Specifies whether log messages will be visible to a client process",
				"enum":        []string{"on", "off"},
				"default":     "off",
			},
			"log_level": map[string]interface{}{
				"type":        "string",
				"description": "Specifies the log level that will be used for log entries",
				"enum":        []string{"debug5", "debug4", "debug3", "debug2", "debug1", "info", "notice", "warning", "log"},
				"default":     "log",
			},
			"log_parameter": map[string]interface{}{
				"type":        "string",
				"description": "Specifies that audit logging should include the parameters that were passed with the statement",
				"enum":        []string{"on", "off"},
				"default":     "off",
			},
			"log_parameter_max_size": map[string]interface{}{
				"type":        "string",
				"description": "Sets the maximum size of a parameter value that will be logged",
				"pattern":     "^[0-9]+[kMGT]?B?$",
				"default":     "0",
			},
			"log_relation": map[string]interface{}{
				"type":        "string",
				"description": "Specifies whether session audit logging should create a separate log entry for each relation referenced in a SELECT or DML statement",
				"enum":        []string{"on", "off"},
				"default":     "off",
			},
			"log_statement": map[string]interface{}{
				"type":        "string",
				"description": "Specifies whether logging will include the statement text and parameters (if enabled)",
				"enum":        []string{"on", "off"},
				"default":     "on",
			},
			"log_statement_once": map[string]interface{}{
				"type":        "string",
				"description": "Specifies whether logging will include the statement text and parameters (if enabled) with the first log entry for a statement/substatement combination or with every log entry",
				"enum":        []string{"on", "off"},
				"default":     "off",
			},
			"role": map[string]interface{}{
				"type":        "string",
				"description": "Specifies the master role to use for object audit logging",
			},
			"session_log_statement_name": map[string]interface{}{
				"type":        "string",
				"description": "Specifies whether the statement name, if provided, should be included in the session log",
				"enum":        []string{"on", "off"},
				"default":     "off",
			},
			"object_log": map[string]interface{}{
				"type":        "string",
				"description": "Specifies which classes of statements will be logged by object audit logging",
				"enum":        []string{"none", "read", "write", "function", "role", "ddl", "misc", "misc_set", "all"},
				"default":     "none",
			},
			"object_log_catalog": map[string]interface{}{
				"type":        "string",
				"description": "Specifies that object logging should be enabled in the case where all relations in a statement are in pg_catalog",
				"enum":        []string{"on", "off"},
				"default":     "on",
			},
			"max_stack_depth": map[string]interface{}{
				"type":        "string",
				"description": "Sets the maximum stack depth for audit logging to prevent infinite recursion",
				"pattern":     "^[0-9]+[kMGT]?B?$",
			},
			"filter_using_role": map[string]interface{}{
				"type":        "string",
				"description": "Specifies whether audit logging should be filtered using role-based access control",
				"enum":        []string{"on", "off"},
				"default":     "off",
			},
		},
	}
}

// getPgHBASchema returns the JSON schema definition for pg_hba.conf configuration
func getPgHBASchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"description":          "PostgreSQL host-based authentication configuration",
		"properties": map[string]interface{}{
			"rules": map[string]interface{}{
				"type":        "array",
				"description": "List of host-based authentication rules",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type": map[string]interface{}{
							"type":        "string",
							"description": "Connection type",
							"enum":        []string{"local", "host", "hostssl", "hostnossl", "hostgssenc", "hostnogssenc"},
						},
						"database": map[string]interface{}{
							"type":        "string",
							"description": "Database name or 'all'",
							"default":     "all",
						},
						"user": map[string]interface{}{
							"type":        "string",
							"description": "Username or 'all'",
							"default":     "all",
						},
						"address": map[string]interface{}{
							"type":        "string",
							"description": "Client IP address, hostname, or CIDR range",
						},
						"method": map[string]interface{}{
							"type":        "string",
							"description": "Authentication method",
							"enum":        []string{"trust", "reject", "md5", "password", "scram-sha-256", "gss", "sspi", "ident", "peer", "ldap", "radius", "cert", "pam", "bsd"},
						},
						"options": map[string]interface{}{
							"type":        "object",
							"description": "Additional authentication options",
							"additionalProperties": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"required": []string{"type", "database", "user", "method"},
				},
			},
		},
	}
}