package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"regexp"
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

	// Generate Go structs from the schema
	fmt.Println("\n🔄 Generating Go structs from schema...")
	if err := generateGoStructs(); err != nil {
		fmt.Printf("Error generating Go structs: %v\n", err)
		os.Exit(1)
	}
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

// TypeHint represents a field that should be converted to a custom type
type TypeHint struct {
	FieldName string
	XType     string
}

// extractTypeHints scans the JSON schema for x-type fields
func extractTypeHints(schemaPath string) (map[string]string, error) {
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file: %w", err)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema: %w", err)
	}

	typeHints := make(map[string]string)
	
	// Navigate through all definitions to find x-type annotations
	if definitions, ok := schema["definitions"].(map[string]interface{}); ok {
		for defName, definition := range definitions {
			if definitionMap, ok := definition.(map[string]interface{}); ok {
				if properties, ok := definitionMap["properties"].(map[string]interface{}); ok {
					for fieldName, fieldDef := range properties {
						if fieldDefMap, ok := fieldDef.(map[string]interface{}); ok {
							if xType, ok := fieldDefMap["x-type"].(string); ok && xType != "" {
								typeHints[fieldName] = xType
								fmt.Printf("Found x-type hint in %s: %s -> %s\n", defName, fieldName, xType)
							}
						}
					}
				}
			}
		}
	}

	return typeHints, nil
}

// runGoJSONSchema executes go-jsonschema to generate the initial struct definitions
func runGoJSONSchema() error {
	fmt.Println("Running go-jsonschema...")
	
	// Check if go-jsonschema is available
	if _, err := exec.LookPath("go-jsonschema"); err != nil {
		// Try to install it
		fmt.Println("Installing go-jsonschema...")
		cmd := exec.Command("go", "install", "github.com/atombender/go-jsonschema@latest")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install go-jsonschema: %w", err)
		}
	}

	// Generate the Go structs
	cmd := exec.Command("go-jsonschema",
		"-p", "pkg",
		"--output", "pkg/model_generated.go",
		"schema/pgconfig-schema.json")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go-jsonschema failed: %w\nOutput: %s", err, output)
	}

	fmt.Println("✅ go-jsonschema completed successfully")
	return nil
}

// postProcessGeneratedCode modifies the generated Go code to use custom pointer types
func postProcessGeneratedCode(filePath string, typeHints map[string]string) error {
	fmt.Println("Post-processing generated code...")

	// Read the generated file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read generated file: %w", err)
	}

	// Parse the Go code
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse generated file: %w", err)
	}

	// Track whether we need to add imports
	needsTypesImport := false

	// Process the AST to replace field types with pointer types and fix validation code
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.StructType:
			for _, field := range node.Fields.List {
				if len(field.Names) == 0 {
					continue // Skip embedded fields
				}
				
				fieldName := field.Names[0].Name
				jsonTag := getJSONTag(field.Tag)
				
				// Check if this field should use a custom type
				if xType, exists := typeHints[jsonTag]; exists {
					var newType string
					switch xType {
					case "Size":
						newType = "*types.Size"
						needsTypesImport = true
					case "Duration":
						newType = "*types.Duration"
						needsTypesImport = true
					default:
						continue // Unknown type, skip
					}
					
					// Replace the type with pointer to custom type
					field.Type = &ast.StarExpr{
						X: &ast.SelectorExpr{
							X:   &ast.Ident{Name: "types"},
							Sel: &ast.Ident{Name: strings.TrimPrefix(strings.TrimPrefix(newType, "*"), "types.")},
						},
					}
					
					fmt.Printf("  Converted field %s (%s) to %s\n", fieldName, jsonTag, newType)
				}
			}
		case *ast.CallExpr:
			// Fix string conversion calls for Size/Duration types only
			if callExpr, ok := node.Fun.(*ast.Ident); ok && callExpr.Name == "string" {
				if len(node.Args) == 1 {
					if starExpr, ok := node.Args[0].(*ast.StarExpr); ok {
						// Check if this is a dereferenced custom type field access
						if selectorExpr, ok := starExpr.X.(*ast.SelectorExpr); ok {
							// Only convert for custom types (field names that have type hints)
							fieldName := selectorExpr.Sel.Name
							
							// Check if this field has a custom type hint
							hasCustomType := false
							for jsonTag, xType := range typeHints {
								// Convert json_tag to field name format (e.g., shared_buffers -> SharedBuffers)
								expectedFieldName := convertToFieldName(jsonTag)
								if fieldName == expectedFieldName && (xType == "Size" || xType == "Duration") {
									hasCustomType = true
									break
								}
							}
							
							if hasCustomType {
								// This looks like *plain.CustomField - convert string(*plain.CustomField) to plain.CustomField.String()
								node.Fun = &ast.SelectorExpr{
									X:   starExpr.X,
									Sel: &ast.Ident{Name: "String"},
								}
								node.Args = nil // Remove arguments since String() takes no args
							}
						}
					}
				}
			}
		}
		return true
	})

	// Add the types import if needed
	if needsTypesImport {
		addImport(file, "github.com/flanksource/postgres/pkg/types")
	}

	// Generate the modified code
	var buf strings.Builder
	if err := format.Node(&buf, fset, file); err != nil {
		return fmt.Errorf("failed to format modified code: %w", err)
	}

	// Write the modified code back to the file
	if err := os.WriteFile(filePath, []byte(buf.String()), 0644); err != nil {
		return fmt.Errorf("failed to write modified file: %w", err)
	}

	fmt.Println("✅ Post-processing completed successfully")
	return nil
}

// getJSONTag extracts the JSON tag value from a struct field tag
func getJSONTag(tag *ast.BasicLit) string {
	if tag == nil {
		return ""
	}
	
	// Remove surrounding backticks
	tagValue := strings.Trim(tag.Value, "`")
	
	// Use regex to extract json tag
	re := regexp.MustCompile(`json:"([^,"]+)`)
	matches := re.FindStringSubmatch(tagValue)
	if len(matches) >= 2 {
		return matches[1]
	}
	
	return ""
}

// convertToFieldName converts a JSON tag to Go field name format
// e.g., "shared_buffers" -> "SharedBuffers"
func convertToFieldName(jsonTag string) string {
	parts := strings.Split(jsonTag, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}

// addImport adds an import to the file if it doesn't already exist
func addImport(file *ast.File, importPath string) {
	// Check if import already exists
	for _, imp := range file.Imports {
		if imp.Path.Value == `"`+importPath+`"` {
			return // Already imported
		}
	}

	// Add the import
	newImport := &ast.ImportSpec{
		Path: &ast.BasicLit{
			Kind:  token.STRING,
			Value: `"` + importPath + `"`,
		},
	}

	if len(file.Imports) == 0 {
		// Create new import declaration
		file.Decls = append([]ast.Decl{&ast.GenDecl{
			Tok:   token.IMPORT,
			Specs: []ast.Spec{newImport},
		}}, file.Decls...)
	} else {
		// Add to existing import declaration
		for _, decl := range file.Decls {
			if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.IMPORT {
				genDecl.Specs = append(genDecl.Specs, newImport)
				break
			}
		}
	}
}

// generateGoStructs generates Go structs from the JSON schema
func generateGoStructs() error {
	// 1. Extract type hints from the schema
	typeHints, err := extractTypeHints("schema/pgconfig-schema.json")
	if err != nil {
		return fmt.Errorf("failed to extract type hints: %w", err)
	}

	fmt.Printf("Found %d fields with type hints\n", len(typeHints))

	// 2. Run go-jsonschema to generate initial Go code
	if err := runGoJSONSchema(); err != nil {
		return fmt.Errorf("failed to run go-jsonschema: %w", err)
	}

	// 3. Post-process the generated code to use pointer custom types
	if err := postProcessGeneratedCode("pkg/model_generated.go", typeHints); err != nil {
		return fmt.Errorf("failed to post-process generated code: %w", err)
	}

	fmt.Println("✅ Successfully generated Go structs with custom pointer types")
	return nil
}
