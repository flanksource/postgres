package pkg

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"reflect"
	"regexp"
	"strconv"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
	"github.com/xeipuuv/gojsonschema"

	"github.com/flanksource/postgres/pkg/schemas"
	"github.com/flanksource/postgres/pkg/types"
	"github.com/flanksource/postgres/pkg/utils"
)

// LoadConfig loads configuration from a YAML file using koanf with schema-based validation and defaults
func LoadConfig(configFile string) (*PgconfigSchemaJson, error) {
	var hasEnvVars bool

	// First validate the raw YAML file against the schema to catch unknown fields
	if configFile != "" {
		// Read and parse YAML file
		yamlData, err := ioutil.ReadFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file %s: %w", configFile, err)
		}

		// Parse YAML to check structure (but not env var resolution)
		parser := yaml.Parser()
		rawConfig, err := parser.Unmarshal(yamlData)
		if err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}

		// Skip schema validation only if the config has environment variable placeholders
		hasEnvVars = containsEnvVarPlaceholders(rawConfig)
		if !hasEnvVars {
			// Validate against schema to catch unknown fields
			if err := validateFileAgainstSchema(configFile); err != nil {
				return nil, err
			}
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
		// If the file contains env var placeholders, we need to expand them
		if hasEnvVars {
			// Read the file, expand env vars, then load
			expandedData, err := expandEnvVarsInYAML(configFile)
			if err != nil {
				return nil, fmt.Errorf("failed to expand env vars in config: %w", err)
			}
			if err := k.Load(rawbytes.Provider(expandedData), yaml.Parser()); err != nil {
				return nil, fmt.Errorf("failed to load config: %w", err)
			}
		} else {
			// No env vars, load directly
			if err := k.Load(file.Provider(configFile), yaml.Parser()); err != nil {
				return nil, fmt.Errorf("failed to load config file %s: %w", configFile, err)
			}
		}
	}

	// Unmarshal configuration into generated struct with mapstructure tags
	var conf PgconfigSchemaJson
	if err := k.UnmarshalWithConf("", &conf, koanf.UnmarshalConf{Tag: "yaml"}); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate against JSON schema
	if err := validateConfiguration(&conf); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
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

// loadSchemaDefaultsSelectively loads schema-defined defaults only for missing keys
func loadSchemaDefaultsSelectively(k *koanf.Koanf) error {
	defaults := utils.GetSchemaDefaults()

	for key, defaultValue := range defaults {
		// Only set defaults for keys that don't already exist
		if !k.Exists(key) {
			// Resolve environment variables in the default value
			resolvedValue := utils.ResolveDefault(defaultValue)
			if resolvedValue != "" {
				// Skip memory parameters if they resolve to invalid formats
				if isMemoryParameter(key) && !isValidMemoryFormat(resolvedValue) {
					continue
				}

				if err := k.Set(key, resolvedValue); err != nil {
					return fmt.Errorf("failed to set default for %s: %w", key, err)
				}
			}
		}
	}

	return nil
}

// isMemoryParameter checks if a parameter is a memory-related parameter
func isMemoryParameter(key string) bool {
	memoryParams := []string{
		"postgres.shared_buffers",
		"postgres.work_mem",
		"postgres.maintenance_work_mem",
		"postgres.effective_cache_size",
		"postgres.wal_buffers",
		"postgres.temp_buffers",
		"pgaudit.log_parameter_max_size",
	}

	for _, param := range memoryParams {
		if key == param {
			return true
		}
	}
	return false
}

// validateConfiguration validates the configuration against the embedded JSON schema
func validateConfiguration(conf *PgconfigSchemaJson) error {
	// Get embedded schema
	schemaData := schemas.GetPgconfigSchemaJSON()

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


// validateFileAgainstSchema validates a YAML file directly against the embedded schema
func validateFileAgainstSchema(configFile string) error {
	// Get embedded schema
	schemaData := schemas.GetPgconfigSchemaJSON()

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
			Name:      field.Name,
			KoanfPath: fieldName,
			EnvVar:    field.Tag.Get("env"),
			Default:   field.Tag.Get("default"),
			Type:      field.Type.String(),
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

// Backward compatibility aliases - the old types now map to the schema-generated types
type Conf = PgconfigSchemaJson
type PostgreSQLConfiguration = PgconfigSchemaJson
type PgBouncerIni = PgBouncerConf
type PostgrestHBA = PgHBAConfRulesElem
type PostgrestHBAType = PgHBAConfRulesElemType
type PostgrestHBAMethod = PgHBAConfRulesElemMethod

// Extension configuration
type ExtensionConfig struct {
	Enabled    bool    `json:"enabled"`
	Version    *string `json:"version,omitempty"`
	Name       string  `json:"name"`
	ConfigFile *string `json:"config_file,omitempty"`
}
type Duration = types.Duration

// containsEnvVarPlaceholders recursively checks if a config contains environment variable placeholders
func containsEnvVarPlaceholders(data interface{}) bool {
	switch v := data.(type) {
	case string:
		// Check if string contains ${...} pattern
		return regexp.MustCompile(`\$\{[^}]+\}`).MatchString(v)
	case map[string]interface{}:
		for _, value := range v {
			if containsEnvVarPlaceholders(value) {
				return true
			}
		}
	case []interface{}:
		for _, item := range v {
			if containsEnvVarPlaceholders(item) {
				return true
			}
		}
	}
	return false
}

// expandEnvVarsInYAML reads a YAML file and expands environment variable placeholders
func expandEnvVarsInYAML(configFile string) ([]byte, error) {
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	// Expand ${VAR} and ${VAR:-default} patterns
	expanded := regexp.MustCompile(`\$\{([A-Z_][A-Z0-9_]*)(:-([^}]+))?\}`).ReplaceAllFunc(data, func(match []byte) []byte {
		s := string(match)
		// Extract variable name and default
		re := regexp.MustCompile(`\$\{([A-Z_][A-Z0-9_]*)(:-([^}]+))?\}`)
		parts := re.FindStringSubmatch(s)
		if len(parts) < 2 {
			return match
		}

		defaultVal := ""
		if len(parts) >= 4 {
			defaultVal = parts[3]
		}

		// Get env var or use default
		if val := utils.ResolveDefault(s); val != "" {
			return []byte(val)
		}
		return []byte(defaultVal)
	})

	return expanded, nil
}

// isValidMemoryFormat checks if a memory value matches the expected pattern
func isValidMemoryFormat(value string) bool {
	pattern := `^[0-9]+[kMGT]?B?$`
	matched, _ := regexp.MatchString(pattern, value)
	return matched
}

// normalizeMemoryValue ensures memory values have proper format
func normalizeMemoryValue(value string) string {
	// If it already matches the pattern, return as-is
	if isValidMemoryFormat(value) {
		return value
	}

	// If it's just a number, assume it's in bytes and add B suffix
	if matched, _ := regexp.MatchString(`^[0-9]+$`, value); matched {
		return value + "B"
	}

	// Return as-is if we can't normalize it
	return value
}
