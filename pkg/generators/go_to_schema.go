package generators

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/flanksource/postgres/pkg"
)

// GoToSchemaGenerator generates JSON schema from Go structs
type GoToSchemaGenerator struct{}

// NewGoToSchemaGenerator creates a new generator that creates JSON schema from Go structs
func NewGoToSchemaGenerator() *GoToSchemaGenerator {
	return &GoToSchemaGenerator{}
}

// GenerateSchema generates a JSON schema from the PgconfigSchemaJson struct
func (g *GoToSchemaGenerator) GenerateSchema() (map[string]interface{}, error) {
	// Generate nested schema for the entire config structure
	schema := map[string]interface{}{
		"$id":                  "https://github.com/flanksource/postgres/schema/pgconfig.json",
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"type":                 "object",
		"title":                "PostgreSQL Stack Configuration",
		"description":          "Configuration for PostgreSQL and related services",
		"additionalProperties": false, // Root level: only allow defined sections
		"properties": map[string]interface{}{
			"postgres":  g.generatePostgresSchema(),
			"pgbouncer": g.generateSchemaForStruct(reflect.TypeOf(pkg.PgBouncerConf{}), false),
			"postgrest": g.generateSchemaForStruct(reflect.TypeOf(pkg.PostgrestConf{}), false),
			"walg":      g.generateSchemaForStruct(reflect.TypeOf(pkg.WalgConf{}), false),
			"pgaudit":   g.generateSchemaForStruct(reflect.TypeOf(pkg.PGAuditConf{}), false),
		},
	}

	return schema, nil
}

// generatePostgresSchema generates schema specifically for PostgresConf with additionalProperties: true
func (g *GoToSchemaGenerator) generatePostgresSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"description":          "PostgreSQL server configuration with commonly used settings",
		"additionalProperties": true, // Allow arbitrary PostgreSQL settings
		"properties":           g.generatePropertiesForType(reflect.TypeOf(pkg.PostgresConf{})),
	}
}

// generateSchemaForStruct generates schema for a struct type with specified additionalProperties setting
func (g *GoToSchemaGenerator) generateSchemaForStruct(t reflect.Type, allowAdditional bool) map[string]interface{} {
	schema := map[string]interface{}{
		"type":                 "object",
		"additionalProperties": allowAdditional,
		"properties":           g.generatePropertiesForType(t),
	}

	// Add description based on struct name
	switch t.Name() {
	case "PgBouncerConf":
		schema["description"] = "PgBouncer connection pooler configuration"
	case "PostgrestConf":
		schema["description"] = "PostgREST RESTful API configuration"
	case "WalgConf":
		schema["description"] = "WAL-G backup and recovery configuration"
	case "PGAuditConf":
		schema["description"] = "PostgreSQL Audit Extension configuration"
	}

	return schema
}

// generatePropertiesForType generates properties for a given type
func (g *GoToSchemaGenerator) generatePropertiesForType(t reflect.Type) map[string]interface{} {
	properties := make(map[string]interface{})

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip fields without JSON tags or marked with "-"
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		// Get field name from JSON tag
		jsonName := strings.Split(jsonTag, ",")[0]
		if jsonName == "-" {
			continue
		}

		// Skip AdditionalProperties field as it's handled by additionalProperties: true
		if jsonName == "" && field.Name == "AdditionalProperties" {
			continue
		}

		// Generate property for this field
		properties[jsonName] = g.generateProperty(field)
	}

	return properties
}

// generateProperty generates a schema property from a struct field
func (g *GoToSchemaGenerator) generateProperty(field reflect.StructField) map[string]interface{} {
	prop := make(map[string]interface{})

	// Get jsonschema tags
	schemaTag := field.Tag.Get("jsonschema")
	if schemaTag != "" {
		// Parse jsonschema tags
		parts := strings.Split(schemaTag, ",")
		for _, part := range parts {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				key := strings.TrimSpace(kv[0])
				value := strings.TrimSpace(kv[1])

				switch key {
				case "description":
					prop["description"] = value
				case "default":
					prop["default"] = g.parseDefault(value, field.Type)
				case "enum":
					if prop["enum"] == nil {
						prop["enum"] = []string{}
					}
					prop["enum"] = append(prop["enum"].([]string), value)
				case "minimum":
					prop["minimum"] = g.parseNumber(value)
				case "maximum":
					prop["maximum"] = g.parseNumber(value)
				case "pattern":
					prop["pattern"] = value
				}
			}
		}
	}

	// Handle type based on field type
	fieldType := field.Type

	// Handle pointer types
	if fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
		// Pointer fields are optional, so we don't add them to required
	}

	// Handle map types (like map[string]DatabaseConfig)
	if fieldType.Kind() == reflect.Map {
		prop["type"] = "object"
		if fieldType.Elem().Kind() == reflect.Struct {
			// For map[string]SomeStruct, generate schema for the value type
			prop["additionalProperties"] = g.generateSchemaForStruct(fieldType.Elem(), false)
		} else {
			prop["additionalProperties"] = map[string]interface{}{
				"type": g.getJSONType(fieldType.Elem()),
			}
		}
		return prop
	}

	// Handle slice types
	if fieldType.Kind() == reflect.Slice {
		prop["type"] = "array"
		elemType := fieldType.Elem()
		if elemType.Kind() == reflect.Struct {
			prop["items"] = g.generateSchemaForStruct(elemType, false)
		} else {
			prop["items"] = map[string]interface{}{
				"type": g.getJSONType(elemType),
			}
		}
		return prop
	}

	// Set type based on Go type
	switch fieldType.String() {
	case "types.Size":
		prop["type"] = "string"
		prop["x-type"] = "Size"
		prop["pattern"] = "^\\d+(\\.\\d+)?\\s*(B|kB|KB|MB|GB|TB)?$"
		if prop["description"] == nil {
			prop["description"] = "Memory or storage size (e.g., '256MB', '1GB')"
		}

	case "types.Duration":
		prop["type"] = "string"
		prop["x-type"] = "Duration"
		prop["pattern"] = "^\\d+(\\.\\d+)?\\s*(us|ms|s|min|h|d)?$"
		if prop["description"] == nil {
			prop["description"] = "Time duration (e.g., '30s', '5min', '1h')"
		}

	default:
		prop["type"] = g.getJSONType(fieldType)
	}

	return prop
}

// getJSONType returns the JSON schema type for a Go type
func (g *GoToSchemaGenerator) getJSONType(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	case reflect.Struct:
		return "object"
	case reflect.Slice, reflect.Array:
		return "array"
	default:
		return "string" // Default to string for unknown types
	}
}

// parseDefault converts a string default value to the appropriate type
func (g *GoToSchemaGenerator) parseDefault(value string, fieldType reflect.Type) interface{} {
	// Handle pointer types
	if fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	}

	switch fieldType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	case reflect.Float32, reflect.Float64:
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f
		}
	case reflect.Bool:
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return value // Return as string if parsing fails or for string types
}

// parseNumber parses a string to a number
func (g *GoToSchemaGenerator) parseNumber(value string) interface{} {
	if i, err := strconv.Atoi(value); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return f
	}
	return 0
}

// GenerateSchemaJSON generates JSON schema and returns it as formatted JSON
func (g *GoToSchemaGenerator) GenerateSchemaJSON() ([]byte, error) {
	schema, err := g.GenerateSchema()
	if err != nil {
		return nil, err
	}

	return json.MarshalIndent(schema, "", "  ")
}

// GenerateAndSaveSchema generates the schema and saves it to a file
func (g *GoToSchemaGenerator) GenerateAndSaveSchema(outputPath string) error {
	schemaJSON, err := g.GenerateSchemaJSON()
	if err != nil {
		return fmt.Errorf("failed to generate schema: %w", err)
	}

	// Write to file (this would use os.WriteFile in real implementation)
	fmt.Printf("Schema would be saved to: %s\n", outputPath)
	fmt.Println(string(schemaJSON))

	return nil
}