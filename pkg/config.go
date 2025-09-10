package pkg

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strconv"

	"github.com/flanksource/postgres/pkg/utils"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/xeipuuv/gojsonschema"
)

// LoadConfig loads configuration from a YAML file using koanf with schema-based validation and defaults
func LoadConfig(configFile string) (*PgconfigSchemaJson, error) {
	k := koanf.New(".")

	// Load schema-based defaults first
	if err := loadSchemaDefaults(k); err != nil {
		return nil, fmt.Errorf("failed to load schema defaults: %w", err)
	}

	// Load environment variables
	if err := k.Load(env.Provider("", ".", func(s string) string {
		return s
	}), nil); err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}

	// Load from file if provided (this will override defaults and env vars)
	if configFile != "" {
		if err := k.Load(file.Provider(configFile), yaml.Parser()); err != nil {
			return nil, fmt.Errorf("failed to load config file %s: %w", configFile, err)
		}
	}

	// Unmarshal configuration into generated struct
	var conf PgconfigSchemaJson
	if err := k.Unmarshal("", &conf); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate against JSON schema
	if err := validateConfiguration(&conf); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &conf, nil
}

// LoadConfigWithValidation loads configuration and validates it against a specific schema file
func LoadConfigWithValidation(configFile, schemaFile string) (*PgconfigSchemaJson, error) {
	// First validate the raw file content against schema (before env var resolution)
	if configFile != "" {
		if err := validateFileAgainstSchema(configFile, schemaFile); err != nil {
			return nil, fmt.Errorf("configuration validation failed: %w", err)
		}
	}

	k := koanf.New(".")

	// Load schema-based defaults first
	if err := loadSchemaDefaults(k); err != nil {
		return nil, fmt.Errorf("failed to load schema defaults: %w", err)
	}

	// Load environment variables
	if err := k.Load(env.Provider("", ".", func(s string) string {
		return s
	}), nil); err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}

	// Load from file if provided (this will override defaults and env vars)
	if configFile != "" {
		if err := k.Load(file.Provider(configFile), yaml.Parser()); err != nil {
			return nil, fmt.Errorf("failed to load config file %s: %w", configFile, err)
		}
	}

	// Unmarshal configuration into generated struct
	var conf PgconfigSchemaJson
	if err := k.Unmarshal("", &conf); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &conf, nil
}

// loadSchemaDefaults loads all schema-defined defaults into koanf
func loadSchemaDefaults(k *koanf.Koanf) error {
	defaults := utils.GetSchemaDefaults()
	
	for key, defaultValue := range defaults {
		// Resolve environment variables in the default value
		resolvedValue := utils.ResolveDefault(defaultValue)
		if resolvedValue != "" {
			if err := k.Set(key, resolvedValue); err != nil {
				return fmt.Errorf("failed to set default for %s: %w", key, err)
			}
		}
	}
	
	return nil
}

// validateConfiguration validates the configuration against the JSON schema
func validateConfiguration(conf *PgconfigSchemaJson) error {
	// Load the JSON schema
	schemaPath := filepath.Join("schema", "pgconfig-schema.json")
	schemaData, err := ioutil.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	// Create schema loader
	schemaLoader := gojsonschema.NewBytesLoader(schemaData)
	
	// Convert configuration to JSON for validation
	configData, err := json.Marshal(conf)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}
	
	// Create config loader
	configLoader := gojsonschema.NewBytesLoader(configData)
	
	// Validate
	result, err := gojsonschema.Validate(schemaLoader, configLoader)
	if err != nil {
		return fmt.Errorf("failed to validate configuration: %w", err)
	}
	
	if !result.Valid() {
		var errMsg string
		for _, desc := range result.Errors() {
			errMsg += fmt.Sprintf("- %s\n", desc)
		}
		return fmt.Errorf("configuration validation errors:\n%s", errMsg)
	}
	
	return nil
}

// validateConfigurationWithSchema validates configuration against a specific schema file
func validateConfigurationWithSchema(conf *PgconfigSchemaJson, schemaFile string) error {
	// Load schema from file
	schemaData, err := ioutil.ReadFile(schemaFile)
	if err != nil {
		return fmt.Errorf("failed to read schema file %s: %w", schemaFile, err)
	}
	
	// Create schema loader
	schemaLoader := gojsonschema.NewBytesLoader(schemaData)
	
	// Convert configuration to JSON for validation
	configData, err := json.Marshal(conf)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}
	
	// Create config loader
	configLoader := gojsonschema.NewBytesLoader(configData)
	
	// Validate
	result, err := gojsonschema.Validate(schemaLoader, configLoader)
	if err != nil {
		return fmt.Errorf("failed to validate configuration: %w", err)
	}
	
	if !result.Valid() {
		var errMsg string
		for _, desc := range result.Errors() {
			errMsg += fmt.Sprintf("- %s\n", desc)
		}
		return fmt.Errorf("configuration validation errors:\n%s", errMsg)
	}
	
	return nil
}

// validateRawConfigurationWithSchema validates the raw koanf configuration against a schema file
func validateRawConfigurationWithSchema(k *koanf.Koanf, schemaFile string) error {
	// Load schema from file
	schemaData, err := ioutil.ReadFile(schemaFile)
	if err != nil {
		return fmt.Errorf("failed to read schema file %s: %w", schemaFile, err)
	}
	
	// Create schema loader
	schemaLoader := gojsonschema.NewBytesLoader(schemaData)
	
	// Get all config as a map for validation
	configData := k.All()
	
	// Convert to JSON for validation
	configJSON, err := json.Marshal(configData)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}
	
	// Create config loader
	configLoader := gojsonschema.NewBytesLoader(configJSON)
	
	// Validate
	result, err := gojsonschema.Validate(schemaLoader, configLoader)
	if err != nil {
		return fmt.Errorf("failed to validate configuration: %w", err)
	}
	
	if !result.Valid() {
		var errMsg string
		for _, desc := range result.Errors() {
			errMsg += fmt.Sprintf("- %s\n", desc)
		}
		return fmt.Errorf("configuration validation errors:\n%s", errMsg)
	}
	
	return nil
}

// validateFileAgainstSchema validates a YAML file directly against a schema file
func validateFileAgainstSchema(configFile, schemaFile string) error {
	// Load schema from file
	schemaData, err := ioutil.ReadFile(schemaFile)
	if err != nil {
		return fmt.Errorf("failed to read schema file %s: %w", schemaFile, err)
	}
	
	// Load and parse the YAML file
	configData, err := ioutil.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", configFile, err)
	}
	
	// Parse YAML to interface{} first
	parser := yaml.Parser()
	yamlData, err := parser.Unmarshal(configData)
	if err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}
	
	// Convert to JSON for validation
	configJSON, err := json.Marshal(yamlData)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}
	
	// Create loaders
	schemaLoader := gojsonschema.NewBytesLoader(schemaData)
	configLoader := gojsonschema.NewBytesLoader(configJSON)
	
	// Validate
	result, err := gojsonschema.Validate(schemaLoader, configLoader)
	if err != nil {
		return fmt.Errorf("failed to validate configuration: %w", err)
	}
	
	if !result.Valid() {
		var errMsg string
		for _, desc := range result.Errors() {
			errMsg += fmt.Sprintf("- %s\n", desc)
		}
		return fmt.Errorf("configuration validation errors:\n%s", errMsg)
	}
	
	return nil
}

// setDefaults sets default values for all struct fields that have a default tag
func setDefaults(v interface{}) error {
	return setDefaultsRecursive(reflect.ValueOf(v).Elem())
}

func setDefaultsRecursive(val reflect.Value) error {
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Handle nested structs
		if field.Kind() == reflect.Struct {
			if err := setDefaultsRecursive(field); err != nil {
				return err
			}
			continue
		}

		// Handle slices of structs
		if field.Kind() == reflect.Slice && field.Type().Elem().Kind() == reflect.Struct {
			// Skip slices for now - they would need special handling
			continue
		}

		// Set default value if tag exists and field is empty
		if defaultVal := fieldType.Tag.Get("default"); defaultVal != "" {
			if isZeroValue(field) {
				if err := setDefaultValue(field, defaultVal); err != nil {
					return fmt.Errorf("failed to set default for field %s: %w", fieldType.Name, err)
				}
			}
		}
	}

	return nil
}

func isZeroValue(val reflect.Value) bool {
	zero := reflect.Zero(val.Type())
	return reflect.DeepEqual(val.Interface(), zero.Interface())
}

func setDefaultValue(field reflect.Value, defaultVal string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(defaultVal)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		val, err := strconv.ParseInt(defaultVal, 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(val)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		val, err := strconv.ParseUint(defaultVal, 10, 64)
		if err != nil {
			return err
		}
		field.SetUint(val)
	case reflect.Float32, reflect.Float64:
		val, err := strconv.ParseFloat(defaultVal, 64)
		if err != nil {
			return err
		}
		field.SetFloat(val)
	case reflect.Bool:
		val, err := strconv.ParseBool(defaultVal)
		if err != nil {
			return err
		}
		field.SetBool(val)
	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}
	return nil
}

// GetAllDefaults extracts all default values from the Conf struct
func GetAllDefaults() map[string]string {
	defaults := make(map[string]string)
	extractDefaults(reflect.TypeOf(Conf{}), "", defaults)
	return defaults
}

func extractDefaults(typ reflect.Type, prefix string, defaults map[string]string) {
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		
		// Skip unexported fields
		if field.PkgPath != "" {
			continue
		}

		koanfTag := field.Tag.Get("koanf")
		if koanfTag == "" {
			continue
		}

		fieldName := koanfTag
		if prefix != "" {
			fieldName = prefix + "." + koanfTag
		}

		// Handle nested structs
		if field.Type.Kind() == reflect.Struct {
			extractDefaults(field.Type, fieldName, defaults)
			continue
		}

		// Handle slices of structs
		if field.Type.Kind() == reflect.Slice && field.Type.Elem().Kind() == reflect.Struct {
			// Skip slices for now
			continue
		}

		// Extract default value
		if defaultVal := field.Tag.Get("default"); defaultVal != "" {
			defaults[fieldName] = defaultVal
		}
	}
}

// GetFieldInfo returns information about all configuration fields
type FieldInfo struct {
	Name        string
	KoanfPath   string
	EnvVar      string
	Default     string
	Description string
	Type        string
}

func GetAllFieldInfo() []FieldInfo {
	var fields []FieldInfo
	extractFieldInfo(reflect.TypeOf(Conf{}), "", &fields)
	return fields
}

func extractFieldInfo(typ reflect.Type, prefix string, fields *[]FieldInfo) {
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		
		// Skip unexported fields
		if field.PkgPath != "" {
			continue
		}

		koanfTag := field.Tag.Get("koanf")
		if koanfTag == "" {
			continue
		}

		fieldName := koanfTag
		if prefix != "" {
			fieldName = prefix + "." + koanfTag
		}

		// Handle nested structs
		if field.Type.Kind() == reflect.Struct {
			extractFieldInfo(field.Type, fieldName, fields)
			continue
		}

		// Handle slices of structs
		if field.Type.Kind() == reflect.Slice && field.Type.Elem().Kind() == reflect.Struct {
			// Skip slices for now
			continue
		}

		// Extract field information
		fieldInfo := FieldInfo{
			Name:        field.Name,
			KoanfPath:   fieldName,
			EnvVar:      field.Tag.Get("env"),
			Default:     field.Tag.Get("default"),
			Type:        field.Type.String(),
		}

		// Extract description from comment (would need to be manually maintained)
		// For now, we'll extract it from the field name and make it human-readable
		fieldInfo.Description = makeDescription(field.Name, fieldName)

		*fields = append(*fields, fieldInfo)
	}
}

func makeDescription(fieldName, koanfPath string) string {
	// This is a simplified approach - in a real implementation,
	// you might want to maintain descriptions separately or use godoc parsing
	switch koanfPath {
	case "pgaudit.log":
		return "Specifies which classes of statements will be logged by session audit logging"
	case "pgaudit.log_catalog":
		return "Specifies that session logging should be enabled when all relations in a statement are in pg_catalog"
	case "pgaudit.log_client":
		return "Specifies whether log messages will be visible to a client process"
	case "pgaudit.log_level":
		return "Specifies the log level for log entries"
	case "pgaudit.log_parameter":
		return "Specifies that audit logging should include statement parameters"
	case "pgaudit.log_parameter_max_size":
		return "Specifies maximum parameter value length for logging"
	case "pgaudit.log_relation":
		return "Specifies whether to create separate log entries for each relation in a statement"
	case "pgaudit.log_rows":
		return "Specifies that audit logging should include number of rows retrieved or affected"
	case "pgaudit.log_statement":
		return "Specifies whether logging will include statement text and parameters"
	case "pgaudit.log_statement_once":
		return "Specifies whether logging will include statement text only once"
	case "pgaudit.role":
		return "Specifies the role for object audit logging"
	default:
		return fmt.Sprintf("Configuration setting for %s", fieldName)
	}
}

// Backward compatibility alias - the old Conf type now maps to the schema-generated PgconfigSchemaJson
type Conf = PgconfigSchemaJson