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
	"strings"

	"github.com/flanksource/postgres/pkg/schemas"
)

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

// SchemaGenerator is a minimal struct to hold parameters
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

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s <describe-config-output-file>\n", os.Args[0])
		fmt.Println("The describe-config-output should be the raw output from PostgreSQL's describe-config command")
		os.Exit(1)
	}

	// Debug: Show current working directory
	pwd, _ := os.Getwd()
	fmt.Printf("generate_schema running from: %s\n", pwd)

	// Read the describe-config output from file
	describeConfigData, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Printf("Error reading describe-config output file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Generating JSON schema from PostgreSQL describe-config...")

	// Parse the describe-config output using the unified parser
	params, err := schemas.ParseDescribeConfig(string(describeConfigData))
	if err != nil {
		fmt.Printf("Error parsing describe-config output: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Parsed %d parameters from describe-config\n", len(params))

	// Create a schema generator with the parsed parameters
	generator, err := NewSchemaGenerator(params, "17.0.0")
	if err != nil {
		fmt.Printf("Error creating schema generator: %v\n", err)
		os.Exit(1)
	}

	// Generate PostgreSQL configuration schema
	postgresProps := make(map[string]*SchemaProperty)
	for _, param := range params {
		prop := convertParamToSchemaProperty(generator, param)
		if prop != nil {
			postgresProps[param.Name] = prop
		}
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
				"properties":           postgresProps,
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

	schemaPath := "pgconfig-schema.json"
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
	fmt.Printf("   PostgreSQL properties: %d\n", len(postgresProps))
	fmt.Printf("   Service definitions: PostgresConf, PgBouncerConf, PostgrestConf, WalgConf, PGAuditConf, PgHBAConf, DatabaseConfig\n")

	// Generate Go structs from the schema
	fmt.Println("\n🔄 Generating Go structs from schema...")
	if err := generateGoStructs(); err != nil {
		fmt.Printf("Error generating Go structs: %v\n", err)
		os.Exit(1)
	}
}

// convertParamToSchemaProperty converts a Param to SchemaProperty using the generator's logic
func convertParamToSchemaProperty(sg *SchemaGenerator, param schemas.Param) *SchemaProperty {
	// Combine short description with extra documentation
	description := param.ShortDesc
	if description == "\\N" {
		description = ""
	}
	if param.ExtraDesc != "" && param.ExtraDesc != "\\N" {
		if description != "" {
			description = description + " " + param.ExtraDesc
		} else {
			description = param.ExtraDesc
		}
	}

	prop := &SchemaProperty{
		Description: description,
	}

	// Detect x-type based on unit
	xType := detectXType(param)
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
		prop.Pattern = getPatternForXType(xType, param.Name, param.Unit)
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
			if param.MinVal != 0 {
				prop.Minimum = param.MinVal
			}
			if param.MaxVal != 0 {
				prop.Maximum = param.MaxVal
			}

		case "real":
			prop.Type = "number"
			if param.MinVal != 0 {
				prop.Minimum = param.MinVal
			}
			if param.MaxVal != 0 {
				prop.Maximum = param.MaxVal
			}

		case "string":
			prop.Type = "string"

			// Handle enum values
			if len(param.EnumVals) > 0 {
				prop.Enum = param.EnumVals
			}

		case "enum":
			prop.Type = "string"
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
		prop.Units = getUnitsDescription(param.Unit)
	}

	// Mark sensitive parameters
	if isSensitiveParam(param.Name) {
		prop.Sensitive = true
	}

	return prop
}

// detectXType determines if a parameter should be treated as a special type based on PostgreSQL metadata
func detectXType(param schemas.Param) string {
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

// getPatternForXType returns regex pattern based on x-type
func getPatternForXType(xType, name, unit string) string {
	switch xType {
	case "Size":
		return "^[0-9]+[kMGT]?B$"
	case "Duration":
		return "^[0-9]+(us|ms|s|min|h|d)?$"
	default:
		return ""
	}
}

// getUnitsDescription returns user-friendly units description
func getUnitsDescription(unit string) string {
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
func isSensitiveParam(name string) bool {
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

// parseDefaultValue parses the default value based on the parameter type
func parseDefaultValue(bootVal, varType string) interface{} {
	if bootVal == "" || bootVal == "\\N" {
		return nil
	}

	switch varType {
	case "bool", "boolean":
		return bootVal == "on" || bootVal == "true" || bootVal == "yes" || bootVal == "1"
	case "integer":
		// Try to parse as integer
		var val int
		fmt.Sscanf(bootVal, "%d", &val)
		return val
	case "real":
		// Try to parse as float
		var val float64
		fmt.Sscanf(bootVal, "%f", &val)
		return val
	default:
		return bootVal
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

		filePath := fmt.Sprintf("%s-schema.json", name)
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
		"--output", "../pkg/model_generated.go",
		"pgconfig-schema.json")
	
	// Add debug output
	cmd.Dir = ""  // Use current directory 
	fmt.Printf("Running go-jsonschema from directory: %s\n", cmd.Dir)
	pwd, _ := os.Getwd()
	fmt.Printf("Current working directory: %s\n", pwd)

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

	// Count structs before processing
	beforeCount := strings.Count(string(content), "type PostgresConf struct")
	postgresFieldsBefore := len(strings.Split(strings.Split(string(content), "type PostgresConf struct")[1], "}")[0])
	fmt.Printf("Before processing: PostgresConf found %d times, approx %d chars\n", beforeCount, postgresFieldsBefore)

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
		case *ast.AssignStmt:
			// Fix assignments to custom type fields
			if len(node.Lhs) == 1 && len(node.Rhs) == 1 {
				if selectorExpr, ok := node.Lhs[0].(*ast.SelectorExpr); ok {
					fieldName := selectorExpr.Sel.Name
					// Check if this field should use a custom type
					for jsonTag, xType := range typeHints {
						expectedFieldName := convertToFieldName(jsonTag)
						if fieldName == expectedFieldName && (xType == "Size" || xType == "Duration") {
							// Check if we're assigning a string literal
							if basicLit, ok := node.Rhs[0].(*ast.BasicLit); ok && basicLit.Kind.String() == "STRING" {
								// Convert assignment from plain.Field = "value" to plain.Field = types.ParseSize("value") or types.ParseDuration("value")
								var parseFunc string
								switch xType {
								case "Size":
									parseFunc = "ParseSize"
								case "Duration":
									parseFunc = "ParseDuration"
								}
								if parseFunc != "" {
									node.Rhs[0] = &ast.CallExpr{
										Fun: &ast.SelectorExpr{
											X:   &ast.Ident{Name: "types"},
											Sel: &ast.Ident{Name: parseFunc},
										},
										Args: []ast.Expr{basicLit},
									}
								}
							}
							break
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

	// Count after processing
	afterCount := strings.Count(buf.String(), "type PostgresConf struct")
	postgresFieldsAfter := 0
	if afterCount > 0 {
		postgresFieldsAfter = len(strings.Split(strings.Split(buf.String(), "type PostgresConf struct")[1], "}")[0])
	}
	fmt.Printf("After processing: PostgresConf found %d times, approx %d chars\n", afterCount, postgresFieldsAfter)

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
	typeHints, err := extractTypeHints("pgconfig-schema.json")
	if err != nil {
		return fmt.Errorf("failed to extract type hints: %w", err)
	}

	fmt.Printf("Found %d fields with type hints\n", len(typeHints))

	// 2. Run go-jsonschema to generate initial Go code
	if err := runGoJSONSchema(); err != nil {
		return fmt.Errorf("failed to run go-jsonschema: %w", err)
	}

	// 3. Post-process the generated code to use custom types - skipping for now
	fmt.Println("✅ Generated Go structs with string types for Size/Duration parameters")
	fmt.Println("   Size/Duration validation will be handled by JSON schema patterns")
	// if err := postProcessGeneratedCode("../pkg/model_generated.go", typeHints); err != nil {
	// 	return fmt.Errorf("failed to post-process generated code: %w", err)
	// }
	return nil
}
