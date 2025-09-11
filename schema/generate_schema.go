package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/flanksource/postgres/pkg/schemas"
)

// SchemaProperty represents a JSON schema property
type SchemaProperty struct {
	Type        string                     `json:"type,omitempty"`
	Description string                     `json:"description,omitempty"`
	Default     interface{}                `json:"default,omitempty"`
	Enum        []interface{}              `json:"enum,omitempty"`
	Pattern     string                     `json:"pattern,omitempty"`
	Minimum     *float64                   `json:"minimum,omitempty"`
	Maximum     *float64                   `json:"maximum,omitempty"`
	XType       string                     `json:"x-type,omitempty"`
	XSensitive  bool                       `json:"x-sensitive,omitempty"`
	Properties  map[string]*SchemaProperty `json:"properties,omitempty"`
	Items       *SchemaProperty            `json:"items,omitempty"`
}

// Param represents a PostgreSQL parameter from describe-config
type Param struct {
	Name           string
	Setting        string
	Unit           string
	Category       string
	Description    string
	ShortDesc      string
	ExtraDesc      string
	Context        string
	Vartype        string
	Source         string
	MinVal         string
	MaxVal         string
	EnumVals       []string
	BootVal        string
	ResetVal       string
	SourceFile     string
	SourceLine     string
	PendingRestart bool
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
			Name:           strings.TrimSpace(parts[0]),
			Setting:        strings.TrimSpace(parts[1]),
			Unit:           strings.TrimSpace(parts[2]),
			Category:       strings.TrimSpace(parts[3]),
			Description:    strings.TrimSpace(parts[4]) + " " + strings.TrimSpace(parts[5]),
			ShortDesc:      strings.TrimSpace(parts[4]),
			ExtraDesc:      strings.TrimSpace(parts[5]),
			Context:        strings.TrimSpace(parts[6]),
			Vartype:        strings.TrimSpace(parts[7]),
			Source:         strings.TrimSpace(parts[8]),
			MinVal:         strings.TrimSpace(parts[9]),
			MaxVal:         strings.TrimSpace(parts[10]),
			EnumVals:       enumVals,
			BootVal:        strings.TrimSpace(parts[11]),
			ResetVal:       strings.TrimSpace(parts[12]),
			SourceFile:     strings.TrimSpace(parts[13]),
			SourceLine:     strings.TrimSpace(parts[14]),
			PendingRestart: strings.TrimSpace(parts[15]) == "t",
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
	if isMemoryParam(param) || param.Unit == "kB" || param.Unit == "8kB" {
		prop.XType = "Size"
		prop.Pattern = "^[0-9]+[kMGT]?B?$"
		prop.Type = "string" // Represent memory sizes as strings
	}

	if isTimeParam(param.Name) || param.Unit == "ms" || param.Unit == "s" || param.Unit == "min" {
		prop.XType = "Duration"
		prop.Type = "string"
	}

	if isPasswordParam(param.Name) {
		prop.XSensitive = true
	}

	return prop
}

// isMemoryParam checks if a parameter is memory-related
func isMemoryParam(p Param) bool {

	if p.Category == "Resource Usage / Memory" && p.Vartype == "integer" {
		return true
	}
	if strings.Contains(strings.ToLower(p.Name), "in bytes") {
		return true
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

	// Get schemas from embedded JSON files
	pgbouncerSchema, err := schemas.GetPgBouncerSchema()
	if err != nil {
		fmt.Printf("Error loading PgBouncer schema: %v\n", err)
		os.Exit(1)
	}

	databaseConfigSchema, err := schemas.GetDatabaseConfigSchema()
	if err != nil {
		fmt.Printf("Error loading DatabaseConfig schema: %v\n", err)
		os.Exit(1)
	}

	postgrestSchema, err := schemas.GetPostgRESTSchema()
	if err != nil {
		fmt.Printf("Error loading PostgREST schema: %v\n", err)
		os.Exit(1)
	}

	walgSchema, err := schemas.GetWalGSchema()
	if err != nil {
		fmt.Printf("Error loading WAL-G schema: %v\n", err)
		os.Exit(1)
	}

	pgauditSchema, err := schemas.GetPGAuditSchema()
	if err != nil {
		fmt.Printf("Error loading PGAudit schema: %v\n", err)
		os.Exit(1)
	}

	pghbaSchema, err := schemas.GetPgHBASchema()
	if err != nil {
		fmt.Printf("Error loading pg_hba schema: %v\n", err)
		os.Exit(1)
	}

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
			"PgBouncerConf":  pgbouncerSchema,
			"DatabaseConfig": databaseConfigSchema,
			"PostgrestConf":  postgrestSchema,
			"WalgConf":       walgSchema,
			"PGAuditConf":    pgauditSchema,
			"PgHBAConf":      pghbaSchema,
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
