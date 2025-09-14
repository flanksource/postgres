package pkg

import (
	"embed"
	"fmt"
	"strconv"
	"strings"
)

//go:embed pgconfig-validator/src/main/resources/guc-info/*.csv
var paramCSVFiles embed.FS

// CSVParam represents a parameter loaded from CSV files
type CSVParam struct {
	Name         string   // Parameter name
	Type         string   // Type: bool, enum, integer, real, string
	DefaultValue string   // Default value
	Unit         string   // Unit: kB, MB, GB, s, ms, min, etc.
	MinValue     string   // Minimum value (for numeric types)
	MaxValue     string   // Maximum value (for numeric types)
	EnumValues   []string // Valid enum values (for enum types)
}

// PgVersion represents a PostgreSQL version for CSV file mapping
type PgVersion string

const (
	PgVersion9_1  PgVersion = "V9_1"
	PgVersion9_2  PgVersion = "V9_2"
	PgVersion9_3  PgVersion = "V9_3"
	PgVersion9_4  PgVersion = "V9_4"
	PgVersion9_5  PgVersion = "V9_5"
	PgVersion9_6  PgVersion = "V9_6"
	PgVersion10   PgVersion = "V10"
	PgVersion11   PgVersion = "V11"
	PgVersion12   PgVersion = "V12"
	PgVersion13   PgVersion = "V13"
	PgVersion14   PgVersion = "V14"
	PgVersion15   PgVersion = "V15"
	PgVersion16   PgVersion = "V16"
	PgVersion17   PgVersion = "V17"
	PgVersion18   PgVersion = "V18"
)

// GetPgVersion maps a PostgreSQL version string to a PgVersion
func GetPgVersion(version string) (PgVersion, error) {
	// Extract major version
	parts := strings.Split(version, ".")
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid version format: %s", version)
	}

	major := parts[0]
	
	// Handle 9.x versions
	if major == "9" && len(parts) >= 2 {
		switch parts[1] {
		case "1":
			return PgVersion9_1, nil
		case "2":
			return PgVersion9_2, nil
		case "3":
			return PgVersion9_3, nil
		case "4":
			return PgVersion9_4, nil
		case "5":
			return PgVersion9_5, nil
		case "6":
			return PgVersion9_6, nil
		default:
			return "", fmt.Errorf("unsupported PostgreSQL 9.x version: %s", version)
		}
	}

	// Handle 10+ versions
	switch major {
	case "10":
		return PgVersion10, nil
	case "11":
		return PgVersion11, nil
	case "12":
		return PgVersion12, nil
	case "13":
		return PgVersion13, nil
	case "14":
		return PgVersion14, nil
	case "15":
		return PgVersion15, nil
	case "16":
		return PgVersion16, nil
	case "17":
		return PgVersion17, nil
	case "18":
		return PgVersion18, nil
	default:
		// Try to parse as integer for future versions
		if majorInt, err := strconv.Atoi(major); err == nil && majorInt > 18 {
			// Default to V18 for newer versions
			return PgVersion18, nil
		}
		return "", fmt.Errorf("unsupported PostgreSQL version: %s", version)
	}
}

// LoadParametersFromCSV loads parameters from the embedded CSV file for the specified version
func LoadParametersFromCSV(version PgVersion) ([]CSVParam, error) {
	filename := fmt.Sprintf("pgconfig-validator/src/main/resources/guc-info/params-%s.csv", version)
	
	data, err := paramCSVFiles.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV file for version %s: %w", version, err)
	}

	return ParseCSVParams(string(data))
}

// ParseCSVParams parses the CSV content into parameters
func ParseCSVParams(content string) ([]CSVParam, error) {
	var params []CSVParam
	lines := strings.Split(content, "\n")

	for lineNum, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse tab-separated values
		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			return nil, fmt.Errorf("invalid CSV format at line %d: expected at least 7 fields, got %d", lineNum+1, len(fields))
		}

		// Parse the parameter
		param := CSVParam{
			Name:         fields[0],
			Type:         fields[1],
			DefaultValue: fromNull(fields[2]),
			Unit:         fromNull(fields[3]),
			MinValue:     fromNull(fields[4]),
			MaxValue:     fromNull(fields[5]),
			EnumValues:   parseCSVEnumValues(fields[6]),
		}

		params = append(params, param)
	}

	return params, nil
}

// fromNull converts \N to empty string (null representation in CSV)
func fromNull(value string) string {
	if value == "\\N" {
		return ""
	}
	return value
}

// parseCSVEnumValues parses enum values from the CSV format
// Format: {val1,val2,val3} or \N for no values
func parseCSVEnumValues(value string) []string {
	if value == "\\N" || value == "" {
		return nil
	}

	// Remove curly braces
	value = strings.Trim(value, "{}")
	if value == "" {
		return nil
	}

	// Split by comma
	values := strings.Split(value, ",")
	
	// Clean up each value
	var result []string
	for _, v := range values {
		v = strings.TrimSpace(v)
		v = strings.Trim(v, "\"") // Remove quotes if present
		if v != "" {
			result = append(result, v)
		}
	}

	return result
}

// ConvertCSVToParam converts a CSVParam to the standard Param structure
func ConvertCSVToParam(csv CSVParam) Param {
	param := Param{
		Name:      csv.Name,
		VarType:   csv.Type,
		Unit:      csv.Unit,
		BootVal:   csv.DefaultValue,
		EnumVals:  csv.EnumValues,
		VarClass:  "configuration", // Default since not in CSV
		Context:   "unknown",        // Will need to be enriched from other sources
		Category:  "",               // Will need to be enriched from other sources
		ShortDesc: "",               // Will need to be enriched from other sources
		ExtraDesc: "",               // Will need to be enriched from other sources
	}

	// Parse min/max values
	if csv.MinValue != "" {
		if val, err := strconv.ParseFloat(csv.MinValue, 64); err == nil {
			param.MinVal = val
		}
	}
	if csv.MaxValue != "" {
		if val, err := strconv.ParseFloat(csv.MaxValue, 64); err == nil {
			param.MaxVal = val
		}
	}

	return param
}

// LoadParametersForVersion loads and converts parameters for a specific PostgreSQL version
func LoadParametersForVersion(version string) ([]Param, error) {
	pgVersion, err := GetPgVersion(version)
	if err != nil {
		return nil, err
	}

	csvParams, err := LoadParametersFromCSV(pgVersion)
	if err != nil {
		return nil, err
	}

	// Convert to standard Param format
	params := make([]Param, len(csvParams))
	for i, csvParam := range csvParams {
		params[i] = ConvertCSVToParam(csvParam)
	}

	return params, nil
}

// GetParameterMap returns a map of parameters by lowercase name for quick lookup
func GetParameterMap(params []Param) map[string]*Param {
	paramMap := make(map[string]*Param)
	for i := range params {
		paramMap[strings.ToLower(params[i].Name)] = &params[i]
	}
	return paramMap
}