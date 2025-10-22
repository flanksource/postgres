package schemas

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Param represents a PostgreSQL configuration parameter with all its metadata
type Param struct {
	Name           string   `json:"name"`            // Parameter name (e.g., "shared_buffers")
	VarType        string   `json:"vartype"`         // Type: bool, enum, integer, real, string
	Unit           string   `json:"unit"`            // Unit: kB, MB, GB, s, ms, min, etc.
	Category       string   `json:"category"`        // Category path (e.g., "Write-Ahead Log / Settings")
	ShortDesc      string   `json:"short_desc"`      // Short description
	ExtraDesc      string   `json:"extra_desc"`      // Extended description
	Context        string   `json:"context"`         // When can be changed: postmaster, sighup, backend, superuser, user
	VarClass       string   `json:"varclass"`        // Class: configuration, preset, fixed
	MinVal         float64  `json:"min_val"`         // Minimum value (for numeric types)
	MaxVal         float64  `json:"max_val"`         // Maximum value (for numeric types)
	EnumVals       []string `json:"enum_vals"`       // Valid enum values (for enum types)
	BootVal        string   `json:"boot_val"`        // Default/boot value
	Setting        string   `json:"setting"`         // Current setting value
	SourceFile     string   `json:"source_file"`     // Configuration source file
	SourceLine     string   `json:"source_line"`     // Line number in source file
	PendingRestart bool     `json:"pending_restart"` // Whether change requires restart
}

// ParseDescribeConfig parses the output of `postgres --describe-config`
// The output is tab-separated values with the following fields (PostgreSQL 17+):
// name, context, category, vartype, boot_val, min_val, max_val, short_desc, extra_desc
func ParseDescribeConfig(output string) ([]Param, error) {
	var params []Param
	lines := strings.Split(output, "\n")

	for lineNum, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse tab-separated values
		fields := strings.Split(line, "\t")
		if len(fields) < 8 {
			return nil, fmt.Errorf("invalid describe-config output at line %d: expected at least 8 fields, got %d", lineNum+1, len(fields))
		}

		// Ensure we have enough fields by padding with empty strings
		for len(fields) < 9 {
			fields = append(fields, "")
		}

		param := Param{
			Name:      fields[0],
			Context:   fields[1],
			Category:  fields[2],
			VarType:   strings.ToLower(fields[3]), // Convert to lowercase (BOOLEAN -> boolean, etc.)
			BootVal:   fields[4],
			MinVal:    parseFloat(fields[5]),
			MaxVal:    parseFloat(fields[6]),
			ShortDesc: fields[7],
			ExtraDesc: fields[8],
			VarClass:  "configuration", // Default since not provided in this format
			Setting:   fields[4],       // Use boot value as default setting
		}
		param.Unit = extractUnit(param)

		// Handle enum values for ENUM type
		if strings.ToLower(param.VarType) == "enum" {
			param.EnumVals = extractEnumValues(param.Name, param.ShortDesc, param.ExtraDesc)
		}

		params = append(params, param)
	}

	return params, nil
}

// extractUnit tries to extract unit information from parameter name and descriptions
func extractUnit(p Param) string {

	if p.Category == "Resource Usage / Memory" && p.VarType == "integer" {
		if p.MaxVal > 128*1024 {
			return "MB"
		}
		return "kB"
	}

	// Check descriptions for unit mentions
	desc := strings.ToLower(p.ShortDesc + " " + p.ExtraDesc)

	// Check for memory-related keywords
	if strings.Contains(desc, "memory") ||
		strings.Contains(desc, "buffer") ||
		strings.Contains(desc, "cache") ||
		strings.Contains(desc, "size of") {
		// Check if it's measured in pages (8kB blocks)
		if strings.Contains(desc, "page") || strings.Contains(desc, "8 kb") {
			return "8kB"
		}
		// Default to kB for memory parameters
		return "kB"
	}

	// Time-related checks
	if strings.Contains(desc, "in milliseconds") || strings.Contains(desc, "timeout") {
		return "ms"
	}
	if strings.Contains(desc, "in seconds") {
		return "s"
	}
	if strings.Contains(desc, "in minutes") {
		return "min"
	}

	// Explicit unit mentions
	if strings.Contains(desc, "kilobytes") {
		return "kB"
	}
	if strings.Contains(desc, "megabytes") {
		return "MB"
	}
	if strings.Contains(desc, "bytes") {
		return "B"
	}
	return ""
}

// extractEnumValues tries to extract enum values from parameter descriptions or known values
func extractEnumValues(name, shortDesc, extraDesc string) []string {
	// Known enum values for common parameters
	knownEnums := map[string][]string{
		"wal_level":                     {"minimal", "replica", "logical"},
		"log_statement":                 {"none", "ddl", "mod", "all"},
		"log_min_messages":              {"debug5", "debug4", "debug3", "debug2", "debug1", "info", "notice", "warning", "error", "log", "fatal", "panic"},
		"client_min_messages":           {"debug5", "debug4", "debug3", "debug2", "debug1", "log", "notice", "warning", "error"},
		"archive_mode":                  {"off", "on", "always"},
		"ssl":                           {"off", "on"},
		"fsync":                         {"off", "on"},
		"synchronous_commit":            {"off", "local", "remote_write", "remote_apply", "on"},
		"checkpoint_completion_target":  {"0.0", "1.0"},
		"default_transaction_isolation": {"serializable", "repeatable read", "read committed", "read uncommitted"},
		"password_encryption":           {"md5", "scram-sha-256"},
		"wal_compression":               {"off", "on", "pglz", "lz4", "zstd"},
		"shared_preload_libraries":      {}, // Complex, leave empty
	}

	if values, exists := knownEnums[name]; exists {
		return values
	}

	// Try to extract from description text
	desc := shortDesc + " " + extraDesc

	// Look for patterns like "Can be 'value1', 'value2', or 'value3'"
	quotedPattern := regexp.MustCompile(`'([^']+)'`)
	matches := quotedPattern.FindAllStringSubmatch(desc, -1)
	if len(matches) > 1 {
		var values []string
		for _, match := range matches {
			if len(match) > 1 {
				values = append(values, match[1])
			}
		}
		return values
	}

	// Look for boolean-like descriptions
	lowerDesc := strings.ToLower(desc)
	if strings.Contains(lowerDesc, "enable") || strings.Contains(lowerDesc, "disable") ||
		strings.Contains(lowerDesc, "on/off") || strings.Contains(lowerDesc, "true/false") {
		return []string{"on", "off"}
	}

	return nil
}

// parseEnumValues parses enum values from the describe-config output (legacy function, kept for compatibility)
// Enum values are typically comma-separated and may be quoted
// Example: "debug5,debug4,debug3,debug2,debug1,info,notice,warning,error,log,fatal,panic"
func parseEnumValues(enumStr string) []string {
	if enumStr == "" || enumStr == "-" {
		return nil
	}

	// Remove surrounding quotes if present
	enumStr = strings.Trim(enumStr, "\"")

	// Split by comma and clean up each value
	values := strings.Split(enumStr, ",")
	var cleanValues []string

	for _, val := range values {
		val = strings.TrimSpace(val)
		if val != "" {
			cleanValues = append(cleanValues, val)
		}
	}

	return cleanValues
}

// GetParamByName returns a parameter by name from a slice of parameters
func GetParamByName(params []Param, name string) *Param {
	for i := range params {
		if params[i].Name == name {
			return &params[i]
		}
	}
	return nil
}

// FilterParamsByCategory returns parameters that belong to the specified category
func FilterParamsByCategory(params []Param, category string) []Param {
	var filtered []Param
	for _, param := range params {
		if strings.Contains(param.Category, category) {
			filtered = append(filtered, param)
		}
	}
	return filtered
}

// FilterParamsByContext returns parameters that can be changed in the specified context
func FilterParamsByContext(params []Param, context string) []Param {
	var filtered []Param
	for _, param := range params {
		if param.Context == context {
			filtered = append(filtered, param)
		}
	}
	return filtered
}

// ValidateParamValue validates a parameter value against its constraints
func (p *Param) ValidateParamValue(value string) error {
	switch p.VarType {
	case "bool":
		return validateBoolValue(value)
	case "enum":
		return validateEnumValue(value, p.EnumVals)
	case "integer":
		return validateIntegerValue(value, p.MinVal, p.MaxVal)
	case "real":
		return validateRealValue(value, p.MinVal, p.MaxVal)
	case "string":
		return nil
	default:
		return fmt.Errorf("unknown parameter type: %s", p.VarType)
	}
}

// validateBoolValue validates a boolean parameter value
func validateBoolValue(value string) error {
	validBools := []string{"on", "off", "true", "false", "yes", "no", "1", "0"}
	lowerValue := strings.ToLower(value)

	for _, valid := range validBools {
		if lowerValue == valid {
			return nil
		}
	}

	return fmt.Errorf("invalid boolean value: %s", value)
}

// validateEnumValue validates an enum parameter value
func validateEnumValue(value string, enumVals []string) error {
	for _, valid := range enumVals {
		if value == valid {
			return nil
		}
	}

	return fmt.Errorf("invalid enum value: %s, valid values: %v", value, enumVals)
}

// validateIntegerValue validates an integer parameter value
func validateIntegerValue(value string, minVal, maxVal float64) error {
	// Basic integer format validation
	matched, err := regexp.MatchString(`^-?\d+$`, value)
	if err != nil {
		return fmt.Errorf("regex error: %w", err)
	}
	if !matched {
		return fmt.Errorf("invalid integer format: %s", value)
	}

	// TODO: Add range validation using minVal and maxVal if they're not empty
	// This would require parsing the values and handling units

	return nil
}

// validateRealValue validates a real/float parameter value
func validateRealValue(value string, minVal, maxVal float64) error {
	// Basic float format validation
	matched, err := regexp.MatchString(`^-?\d*\.?\d+([eE][+-]?\d+)?$`, value)
	if err != nil {
		return fmt.Errorf("regex error: %w", err)
	}
	if !matched {
		return fmt.Errorf("invalid real format: %s", value)
	}

	// TODO: Add range validation using minVal and maxVal if they're not empty

	return nil
}

func parseFloat(s string) float64 {
	val, _ := strconv.ParseFloat(s, 64)
	return val
}
