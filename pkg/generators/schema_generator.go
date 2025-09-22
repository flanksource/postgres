package generators

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/flanksource/postgres/pkg/schemas"
)

// SchemaGenerator generates JSON schema from PostgreSQL describe-config output
type SchemaGenerator struct {
	params  []schemas.Param
	version string
}

// NewSchemaGenerator creates a new schema generator
func NewSchemaGenerator(params []schemas.Param, version string) (*SchemaGenerator, error) {
	return &SchemaGenerator{
		params:  params,
		version: version,
	}, nil
}

// SchemaProperty represents a JSON schema property
type SchemaProperty struct {
	Type           interface{} `json:"type,omitempty"`
	Description    string      `json:"description"`
	Default        interface{} `json:"default,omitempty"`
	Minimum        interface{} `json:"minimum,omitempty"`
	Maximum        interface{} `json:"maximum,omitempty"`
	Pattern        string      `json:"pattern,omitempty"`
	Enum           []string    `json:"enum,omitempty"`
	Documentation  string      `json:"x-documentation,omitempty"`
	Recommendation string      `json:"x-recommendation,omitempty"`
	Units          string      `json:"x-units,omitempty"`
	Sensitive      bool        `json:"x-sensitive,omitempty"`
	XType          string      `json:"x-type,omitempty"`
}

// GeneratePostgresSchema generates PostgreSQL configuration schema from describe-config
func (sg *SchemaGenerator) GeneratePostgresSchema() (map[string]*SchemaProperty, error) {
	params := sg.params

	properties := make(map[string]*SchemaProperty)

	for _, param := range params {
		prop := sg.convertParamToProperty(param)
		if prop != nil {
			properties[param.Name] = prop
		}
	}

	return properties, nil
}

// convertParamToProperty converts a PostgreSQL parameter to a JSON schema property
func (sg *SchemaGenerator) convertParamToProperty(param schemas.Param) *SchemaProperty {
	// Combine short description with extra documentation
	description := param.ShortDesc
	if param.ExtraDesc != "" {
		description = description + " " + param.ExtraDesc
	}

	prop := &SchemaProperty{
		Description: description,
	}

	// Auto-detect type hints from PostgreSQL metadata first
	xType := sg.detectXType(param)
	if xType != "" {
		prop.XType = xType
	}

	// Handle parameter types - Size and Duration parameters should always be strings
	if xType == "Size" || xType == "Duration" {
		prop.Type = "string"
		// Set default value as string for Size/Duration parameters
		if param.BootVal != "" {
			prop.Default = param.BootVal
		}
		// Add patterns for Size/Duration parameters
		prop.Pattern = sg.getPatternForXType(xType, param.Name, param.Unit)
	} else {
		// Set default value with proper type parsing for non-Size/Duration parameters
		if param.BootVal != "" {
			prop.Default = parseDefaultValue(param.BootVal, param.VarType)
		}

		// Handle different parameter types
		switch param.VarType {
		case "bool", "boolean":
			prop.Type = "boolean"

		case "integer":
			prop.Type = "integer"
			prop.Minimum = param.MinVal
			prop.Maximum = param.MaxVal

		case "real":
			prop.Type = "number"
			prop.Minimum = param.MinVal
			prop.Maximum = param.MaxVal

		case "string":
			prop.Type = "string"

			// Handle enum values
			if len(param.EnumVals) > 0 {
				prop.Enum = param.EnumVals
			}

		default:
			// Default to string for unknown types
			prop.Type = "string"
		}
	}

	// Add units information
	if param.Unit != "" {
		prop.Units = sg.getUnitsDescription(param.Unit)
	}

	// Mark sensitive parameters
	if sg.isSensitiveParam(param.Name) {
		prop.Sensitive = true
	}

	return prop
}

// getPatternForXType returns regex pattern based on x-type
func (sg *SchemaGenerator) getPatternForXType(xType, name, unit string) string {
	switch xType {
	case "Size":
		return "^[0-9]+[kMGT]?B$"
	case "Duration":
		return "^[0-9]+(us|ms|s|min|h|d)?$"
	default:
		return ""
	}
}

// getPatternForParam returns regex pattern for specific parameter types (legacy function)
func (sg *SchemaGenerator) getPatternForParam(name, unit string) string {
	// Memory parameters
	if unit == "kB" || unit == "MB" || unit == "GB" {
		return "^[0-9]+[kMGT]?B$"
	}

	// Time parameters
	if unit == "ms" || unit == "s" || unit == "min" || unit == "h" {
		return "^[0-9]+(us|ms|s|min|h|d)?$"
	}

	return ""
}

// getUnitsDescription returns user-friendly units description
func (sg *SchemaGenerator) getUnitsDescription(unit string) string {
	switch unit {
	case "kB", "MB", "GB", "TB":
		return "B, kB, MB, GB, TB (1024 multiplier)"
	case "ms":
		return "us, ms, s, min, h, d"
	case "s":
		return "s, min, h, d"
	default:
		return unit
	}
}

// isSensitiveParam returns true if parameter contains sensitive information
func (sg *SchemaGenerator) isSensitiveParam(name string) bool {
	sensitivePatterns := []string{
		"password",
		"secret",
		"key",
		"token",
		"credential",
	}

	lowerName := strings.ToLower(name)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerName, pattern) {
			return true
		}
	}

	return false
}

// detectXType determines if a parameter should be treated as a special type based on PostgreSQL metadata
func (sg *SchemaGenerator) detectXType(param schemas.Param) string {
	// Exclude parameters that should remain as regular types
	excludeFromXType := []string{
		"checkpoint_completion_target", // Float between 0-1, not a size
	}
	for _, excluded := range excludeFromXType {
		if param.Name == excluded {
			return ""
		}
	}

	// First check the unit field from describe-config - this is the most reliable indicator
	switch param.Unit {
	case "kB", "MB", "GB", "TB":
		return "Size"
	case "8kB": // PostgreSQL block size units
		return "Size"
	case "ms", "s", "min", "h", "d":
		return "Duration"
	}

	// Also check if it's a memory parameter based on category
	if param.Category == "Resource Usage / Memory" && param.VarType == "integer" {
		return "Size"
	}

	// Add specific memory-related parameters that should be Size type
	// These parameters commonly accept size values (e.g., "4GB", "16MB") in configurations
	memoryParams := []string{
		"effective_cache_size",      // Planner memory assumption
		"wal_buffers",              // WAL memory buffers 
		"temp_buffers",             // Temporary buffer memory
		"min_dynamic_shared_memory", // Minimum dynamic shared memory
		"huge_page_size",           // Huge page size
		"log_temp_files",           // Log temporary files above this size
		"logical_decoding_work_mem", // Memory for logical decoding
		"max_stack_depth",          // Maximum stack depth
		"vacuum_buffer_usage_limit", // Vacuum buffer usage limit
		"backend_flush_after",      // Backend flush after pages
		"gin_pending_list_limit",   // GIN pending list size limit
		"log_rotation_size",        // Log rotation size
		"temp_file_limit",          // Temporary file size limit
		"max_slot_wal_keep_size",   // Max WAL kept for slots
		"wal_keep_size",            // WAL keep size
		"autovacuum_work_mem",      // Autovacuum working memory
		"maintenance_work_mem",     // Maintenance work memory
		"shared_buffers",           // Shared buffer pool size
		"work_mem",                 // Working memory for sorts/hashes
	}
	for _, mp := range memoryParams {
		if param.Name == mp {
			return "Size"
		}
	}

	// Check parameter names for time-related parameters
	timeParams := []string{
		"statement_timeout", "lock_timeout", "idle_in_transaction_session_timeout",
		"checkpoint_timeout", "wal_receiver_timeout", "wal_sender_timeout",
		"deadlock_timeout", "authentication_timeout", "tcp_user_timeout",
		"idle_session_timeout", "transaction_timeout", "archive_timeout",
		"log_autovacuum_min_duration", "log_min_duration_statement",
		"autovacuum_vacuum_cost_delay", "vacuum_cost_delay",
		"log_rotation_age", "client_connection_check_interval",
		"tcp_keepalives_idle", "tcp_keepalives_interval", "checkpoint_warning",
		"bgwriter_delay", "wal_writer_delay",
	}
	for _, tp := range timeParams {
		if param.Name == tp {
			return "Duration"
		}
	}

	// Heuristic detection based on parameter names for size-related parameters
	paramName := strings.ToLower(param.Name)
	if strings.Contains(paramName, "buffer") || 
	   strings.Contains(paramName, "_mem") ||
	   strings.Contains(paramName, "mem_") ||
	   strings.Contains(paramName, "memory") ||
	   strings.Contains(paramName, "cache") ||
	   (strings.Contains(paramName, "size") && param.VarType == "integer") {
		return "Size"
	}

	// Heuristic detection for duration parameters
	if strings.Contains(paramName, "timeout") ||
	   strings.Contains(paramName, "delay") ||
	   strings.Contains(paramName, "duration") ||
	   strings.Contains(paramName, "interval") ||
	   strings.Contains(paramName, "age") ||
	   strings.Contains(paramName, "warning") {
		return "Duration"
	}

	return ""
}

// GenerateCompleteSchema generates a complete JSON schema with all sections
func (sg *SchemaGenerator) GenerateCompleteSchema() (map[string]interface{}, error) {
	// Generate PostgreSQL properties
	postgresProps, err := sg.GeneratePostgresSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to generate postgres schema: %w", err)
	}

	// Load existing schema to get other sections (pgbouncer, walg, etc.)
	existingSchema, err := sg.loadExistingSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to load existing schema: %w", err)
	}

	// Update the PostgreSQL section with generated properties
	if definitions, ok := existingSchema["definitions"].(map[string]interface{}); ok {
		if postgresConf, ok := definitions["PostgresConf"].(map[string]interface{}); ok {
			if properties, ok := postgresConf["properties"].(map[string]interface{}); ok {
				// Replace with generated properties
				for name, prop := range postgresProps {
					properties[name] = prop
				}
			}
		}
	}

	return existingSchema, nil
}

// loadExistingSchema loads the existing schema file
func (sg *SchemaGenerator) loadExistingSchema() (map[string]interface{}, error) {
	schemaPath := filepath.Join("schema", "pgconfig-schema.json")

	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file: %w", err)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema: %w", err)
	}

	return schema, nil
}

// WriteSchemaFile writes the generated schema to a file
func (sg *SchemaGenerator) WriteSchemaFile(outputPath string) error {
	schema, err := sg.GenerateCompleteSchema()
	if err != nil {
		return fmt.Errorf("failed to generate schema: %w", err)
	}

	// Pretty print JSON
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write schema file: %w", err)
	}

	return nil
}

// GenerateParameterReport generates a report of all PostgreSQL parameters
func (sg *SchemaGenerator) GenerateParameterReport() (string, error) {
	params := sg.params

	// Sort parameters by category and name
	sort.Slice(params, func(i, j int) bool {
		if params[i].Category != params[j].Category {
			return params[i].Category < params[j].Category
		}
		return params[i].Name < params[j].Name
	})

	var report strings.Builder
	report.WriteString(fmt.Sprintf("# PostgreSQL %s Configuration Parameters\n\n", sg.version))
	report.WriteString(fmt.Sprintf("Generated from postgres --describe-config output\n\n"))
	report.WriteString(fmt.Sprintf("Total parameters: %d\n\n", len(params)))

	currentCategory := ""
	for _, param := range params {
		if param.Category != currentCategory {
			currentCategory = param.Category
			report.WriteString(fmt.Sprintf("## %s\n\n", currentCategory))
		}

		report.WriteString(fmt.Sprintf("### %s\n\n", param.Name))
		report.WriteString(fmt.Sprintf("- **Type**: %s\n", param.VarType))
		if param.Unit != "" {
			report.WriteString(fmt.Sprintf("- **Unit**: %s\n", param.Unit))
		}
		report.WriteString(fmt.Sprintf("- **Context**: %s\n", param.Context))
		if param.BootVal != "" {
			report.WriteString(fmt.Sprintf("- **Default**: %s\n", param.BootVal))
		}

		if len(param.EnumVals) > 0 {
			report.WriteString(fmt.Sprintf("- **Values**: %s\n", strings.Join(param.EnumVals, ", ")))
		}
		report.WriteString(fmt.Sprintf("- **Description**: %s\n", param.ShortDesc))
		if param.ExtraDesc != "" {
			report.WriteString(fmt.Sprintf("- **Details**: %s\n", param.ExtraDesc))
		}
		report.WriteString("\n")
	}

	return report.String(), nil
}

// ValidateExistingSchema compares existing schema with generated schema
func (sg *SchemaGenerator) ValidateExistingSchema() ([]string, error) {
	generatedProps, err := sg.GeneratePostgresSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to generate schema: %w", err)
	}

	existingSchema, err := sg.loadExistingSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to load existing schema: %w", err)
	}

	var issues []string

	// Extract existing PostgreSQL properties
	existingProps := make(map[string]interface{})
	if definitions, ok := existingSchema["definitions"].(map[string]interface{}); ok {
		if postgresConf, ok := definitions["PostgresConf"].(map[string]interface{}); ok {
			if properties, ok := postgresConf["properties"].(map[string]interface{}); ok {
				existingProps = properties
			}
		}
	}

	// Check for missing parameters in existing schema
	for name := range generatedProps {
		if _, exists := existingProps[name]; !exists {
			issues = append(issues, fmt.Sprintf("Missing parameter in existing schema: %s", name))
		}
	}

	// Check for obsolete parameters in existing schema
	for name := range existingProps {
		if _, exists := generatedProps[name]; !exists {
			issues = append(issues, fmt.Sprintf("Obsolete parameter in existing schema: %s", name))
		}
	}

	return issues, nil
}

// parseDefaultValue converts a string default value to the appropriate type
func parseDefaultValue(bootVal, varType string) interface{} {
	switch varType {
	case "bool", "boolean":
		if val, err := strconv.ParseBool(bootVal); err == nil {
			return val
		}
		return false // Default fallback

	case "integer":
		if val, err := strconv.Atoi(bootVal); err == nil {
			return val
		}
		return 0 // Default fallback

	case "real":
		if val, err := strconv.ParseFloat(bootVal, 64); err == nil {
			return val
		}
		return 0.0 // Default fallback

	default:
		// For string types (including enums), return as-is
		return bootVal
	}
}
