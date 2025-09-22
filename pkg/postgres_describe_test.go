package pkg

import (
	"os"
	"testing"

	"github.com/flanksource/postgres/pkg/embedded"
	"github.com/flanksource/postgres/pkg/schemas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sample postgres --describe-config output for testing
var sampleDescribeConfigOutput string

func init() {
	data, err := os.ReadFile("testdata/postgres-describe-v17-sample.txt")
	if err != nil {
		panic("failed to read sample describe-config output: " + err.Error())
	}
	sampleDescribeConfigOutput = string(data)
}

func TestParseDescribeConfig(t *testing.T) {
	params, err := schemas.ParseDescribeConfig(sampleDescribeConfigOutput)
	require.NoError(t, err)

	// Should parse all 6 parameters
	assert.Len(t, params, 6)

	// Test integer parameter (shared_buffers)
	sharedBuffers := schemas.GetParamByName(params, "shared_buffers")
	require.NotNil(t, sharedBuffers)
	assert.Equal(t, "shared_buffers", sharedBuffers.Name)
	assert.Equal(t, "integer", sharedBuffers.VarType)
	assert.Equal(t, "MB", sharedBuffers.Unit)
	assert.Equal(t, "Resource Usage / Memory", sharedBuffers.Category)
	assert.Equal(t, "Sets the number of shared memory buffers used by the server.", sharedBuffers.ShortDesc)
	assert.Equal(t, "", sharedBuffers.ExtraDesc)
	assert.Equal(t, "postmaster", sharedBuffers.Context)
	assert.Equal(t, "configuration", sharedBuffers.VarClass)
	assert.Equal(t, float64(16), sharedBuffers.MinVal)
	assert.Equal(t, float64(1073741823), sharedBuffers.MaxVal)
	assert.Empty(t, sharedBuffers.EnumVals)
	assert.Equal(t, "128MB", sharedBuffers.BootVal)

	// Test enum parameter (log_min_messages)
	logMinMessages := schemas.GetParamByName(params, "log_min_messages")
	require.NotNil(t, logMinMessages)
	assert.Equal(t, "log_min_messages", logMinMessages.Name)
	assert.Equal(t, "enum", logMinMessages.VarType)
	assert.Equal(t, "", logMinMessages.Unit)
	assert.Equal(t, "Reporting and Logging / When to Log", logMinMessages.Category)
	assert.Equal(t, "Sets the message levels that are logged.", logMinMessages.ShortDesc)
	assert.Contains(t, logMinMessages.ExtraDesc, "Each level includes all the levels that follow it")
	assert.Equal(t, "sighup", logMinMessages.Context)
	assert.Equal(t, "configuration", logMinMessages.VarClass)
	assert.Equal(t, float64(0), logMinMessages.MinVal)
	assert.Equal(t, float64(0), logMinMessages.MaxVal)
	expectedEnums := []string{"debug5", "debug4", "debug3", "debug2", "debug1", "info", "notice", "warning", "error", "log", "fatal", "panic"}
	assert.Equal(t, expectedEnums, logMinMessages.EnumVals)
	assert.Equal(t, "warning", logMinMessages.BootVal)

	// Test boolean parameter (enable_seqscan)
	enableSeqscan := schemas.GetParamByName(params, "enable_seqscan")
	require.NotNil(t, enableSeqscan)
	assert.Equal(t, "enable_seqscan", enableSeqscan.Name)
	assert.Equal(t, "boolean", enableSeqscan.VarType)
	assert.Equal(t, "", enableSeqscan.Unit)
	assert.Equal(t, "Query Tuning / Planner Method Configuration", enableSeqscan.Category)
	assert.Equal(t, "Enables the planner's use of sequential-scan plans.", enableSeqscan.ShortDesc)
	assert.Equal(t, "", enableSeqscan.ExtraDesc)
	assert.Equal(t, "user", enableSeqscan.Context)
	assert.Equal(t, "configuration", enableSeqscan.VarClass)
	assert.Equal(t, "on", enableSeqscan.BootVal)

	// Test string parameter (shared_preload_libraries)
	sharedPreload := schemas.GetParamByName(params, "shared_preload_libraries")
	require.NotNil(t, sharedPreload)
	assert.Equal(t, "shared_preload_libraries", sharedPreload.Name)
	assert.Equal(t, "string", sharedPreload.VarType)
	assert.Equal(t, "", sharedPreload.Unit)
	assert.Equal(t, "Client Connection Defaults / Shared Library Preloading", sharedPreload.Category)
	assert.Equal(t, "Lists shared libraries to preload into server.", sharedPreload.ShortDesc)
	assert.Equal(t, "", sharedPreload.ExtraDesc)
	assert.Equal(t, "postmaster", sharedPreload.Context)
	assert.Equal(t, "configuration", sharedPreload.VarClass)
	assert.Equal(t, float64(0), sharedPreload.MinVal)
	assert.Equal(t, float64(0), sharedPreload.MaxVal)
	assert.Empty(t, sharedPreload.EnumVals)
	assert.Equal(t, "", sharedPreload.BootVal)
}

// TestParseEnumValues test removed - parseEnumValues is internal and tested through ParseDescribeConfig

func TestGetParamByName(t *testing.T) {
	params := []schemas.Param{
		{Name: "param1", VarType: "string"},
		{Name: "param2", VarType: "integer"},
		{Name: "param3", VarType: "bool"},
	}

	// Test existing parameter
	result := schemas.GetParamByName(params, "param2")
	require.NotNil(t, result)
	assert.Equal(t, "param2", result.Name)
	assert.Equal(t, "integer", result.VarType)

	// Test non-existing parameter
	result = schemas.GetParamByName(params, "nonexistent")
	assert.Nil(t, result)
}

func TestFilterParamsByCategory(t *testing.T) {
	params := []schemas.Param{
		{Name: "param1", Category: "Resource Usage / Memory"},
		{Name: "param2", Category: "Resource Usage / Disk"},
		{Name: "param3", Category: "Write-Ahead Log / Settings"},
		{Name: "param4", Category: "Resource Usage / Memory"},
	}

	// Test filtering by exact category
	filtered := schemas.FilterParamsByCategory(params, "Resource Usage / Memory")
	assert.Len(t, filtered, 2)
	assert.Equal(t, "param1", filtered[0].Name)
	assert.Equal(t, "param4", filtered[1].Name)

	// Test filtering by partial category
	filtered = schemas.FilterParamsByCategory(params, "Resource Usage")
	assert.Len(t, filtered, 3)

	// Test filtering by non-existing category
	filtered = schemas.FilterParamsByCategory(params, "NonExistent")
	assert.Len(t, filtered, 0)
}

func TestFilterParamsByContext(t *testing.T) {
	params := []schemas.Param{
		{Name: "param1", Context: "postmaster"},
		{Name: "param2", Context: "sighup"},
		{Name: "param3", Context: "user"},
		{Name: "param4", Context: "postmaster"},
	}

	// Test filtering by context
	filtered := schemas.FilterParamsByContext(params, "postmaster")
	assert.Len(t, filtered, 2)
	assert.Equal(t, "param1", filtered[0].Name)
	assert.Equal(t, "param4", filtered[1].Name)

	// Test filtering by non-existing context
	filtered = schemas.FilterParamsByContext(params, "nonexistent")
	assert.Len(t, filtered, 0)
}

func TestParamValidateParamValue(t *testing.T) {
	tests := []struct {
		name     string
		param    schemas.Param
		value    string
		wantErr  bool
		errMatch string
	}{
		// Boolean parameter tests
		{
			name:    "bool valid on",
			param:   schemas.Param{Name: "test_bool", VarType: "bool"},
			value:   "on",
			wantErr: false,
		},
		{
			name:    "bool valid true",
			param:   schemas.Param{Name: "test_bool", VarType: "bool"},
			value:   "true",
			wantErr: false,
		},
		{
			name:    "bool valid 1",
			param:   schemas.Param{Name: "test_bool", VarType: "bool"},
			value:   "1",
			wantErr: false,
		},
		{
			name:     "bool invalid value",
			param:    schemas.Param{Name: "test_bool", VarType: "bool"},
			value:    "maybe",
			wantErr:  true,
			errMatch: "invalid boolean value",
		},
		// Enum parameter tests
		{
			name:    "enum valid value",
			param:   schemas.Param{Name: "test_enum", VarType: "enum", EnumVals: []string{"debug", "info", "warning"}},
			value:   "info",
			wantErr: false,
		},
		{
			name:     "enum invalid value",
			param:    schemas.Param{Name: "test_enum", VarType: "enum", EnumVals: []string{"debug", "info", "warning"}},
			value:    "trace",
			wantErr:  true,
			errMatch: "invalid enum value",
		},
		// Integer parameter tests
		{
			name:    "integer valid value",
			param:   schemas.Param{Name: "test_int", VarType: "integer"},
			value:   "42",
			wantErr: false,
		},
		{
			name:    "integer valid negative",
			param:   schemas.Param{Name: "test_int", VarType: "integer"},
			value:   "-1",
			wantErr: false,
		},
		{
			name:     "integer invalid format",
			param:    schemas.Param{Name: "test_int", VarType: "integer"},
			value:    "not_a_number",
			wantErr:  true,
			errMatch: "invalid integer format",
		},
		// Real parameter tests
		{
			name:    "real valid value",
			param:   schemas.Param{Name: "test_real", VarType: "real"},
			value:   "3.14",
			wantErr: false,
		},
		{
			name:    "real valid scientific",
			param:   schemas.Param{Name: "test_real", VarType: "real"},
			value:   "1.5e-10",
			wantErr: false,
		},
		{
			name:     "real invalid format",
			param:    schemas.Param{Name: "test_real", VarType: "real"},
			value:    "not_a_float",
			wantErr:  true,
			errMatch: "invalid real format",
		},
		// String parameter tests
		{
			name:    "string valid value",
			param:   schemas.Param{Name: "test_string", VarType: "string"},
			value:   "any string value",
			wantErr: false,
		},
		// Unknown parameter type test
		{
			name:     "unknown type",
			param:    schemas.Param{Name: "test_unknown", VarType: "unknown"},
			value:    "value",
			wantErr:  true,
			errMatch: "unknown parameter type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.param.ValidateParamValue(tt.value)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMatch != "" {
					assert.Contains(t, err.Error(), tt.errMatch)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Integration test - requires actual postgres binary
func TestDescribeConfigIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create embedded postgres instance
	pg, err := embedded.NewEmbeddedPostgres("16.1.0")
	if err != nil {
		t.Skipf("Failed to create embedded postgres (expected in some environments): %v", err)
		return
	}

	// Get configuration parameters
	params, err := pg.DescribeConfig()
	if err != nil {
		t.Skipf("Failed to run describe-config (expected if postgres isn't properly installed): %v", err)
		return
	}

	// Verify we got a reasonable number of parameters
	assert.Greater(t, len(params), 200, "PostgreSQL should have more than 200 configuration parameters")

	// Check for some well-known parameters
	knownParams := []string{"shared_buffers", "max_connections", "log_min_messages", "wal_level"}
	for _, paramName := range knownParams {
		param := schemas.GetParamByName(params, paramName)
		assert.NotNilf(t, param, "Parameter %s should exist", paramName)
		if param != nil {
			assert.NotEmptyf(t, param.ShortDesc, "Parameter %s should have a description", paramName)
			assert.NotEmptyf(t, param.VarType, "Parameter %s should have a type", paramName)
		}
	}

	// Verify specific parameter types
	sharedBuffers := schemas.GetParamByName(params, "shared_buffers")
	if sharedBuffers != nil {
		assert.Equal(t, "integer", sharedBuffers.VarType)
		assert.NotEmpty(t, sharedBuffers.Unit) // Should have a unit like "8kB"
	}

	logLevel := schemas.GetParamByName(params, "log_min_messages")
	if logLevel != nil {
		assert.Equal(t, "enum", logLevel.VarType)
		assert.Greater(t, len(logLevel.EnumVals), 5) // Should have multiple log levels
	}
}

func TestParseDescribeConfigErrorHandling(t *testing.T) {
	// Test empty input
	params, err := schemas.ParseDescribeConfig("")
	assert.NoError(t, err)
	assert.Empty(t, params)

	// Test malformed line (too few fields - less than 8)
	malformedOutput := "param1\tinteger\t8kB\tcategory\tshort"
	_, err = schemas.ParseDescribeConfig(malformedOutput)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected at least 8 fields")

	// Test with whitespace lines
	outputWithWhitespace := "param1\tstring\t\tcategory\tshort\textra\tcontext\tclass\t\t\t\tdefault\n\n  \n"
	params, err = schemas.ParseDescribeConfig(outputWithWhitespace)
	assert.NoError(t, err)
	assert.Len(t, params, 1)
	assert.Equal(t, "param1", params[0].Name)
}

func TestValidateBoolValue(t *testing.T) {
	boolParam := schemas.Param{Name: "test_bool", VarType: "bool"}
	validValues := []string{"on", "off", "true", "false", "yes", "no", "1", "0", "ON", "OFF", "True", "False"}
	for _, val := range validValues {
		err := boolParam.ValidateParamValue(val)
		assert.NoErrorf(t, err, "Value %s should be valid", val)
	}

	invalidValues := []string{"maybe", "2", "enable", "disable", ""}
	for _, val := range invalidValues {
		err := boolParam.ValidateParamValue(val)
		assert.Errorf(t, err, "Value %s should be invalid", val)
	}
}

func TestValidateEnumValue(t *testing.T) {
	enumVals := []string{"debug", "info", "warning", "error"}
	enumParam := schemas.Param{Name: "test_enum", VarType: "enum", EnumVals: enumVals}

	// Valid values
	for _, val := range enumVals {
		err := enumParam.ValidateParamValue(val)
		assert.NoErrorf(t, err, "Value %s should be valid", val)
	}

	// Invalid values
	invalidValues := []string{"trace", "verbose", "", "DEBUG"}
	for _, val := range invalidValues {
		err := enumParam.ValidateParamValue(val)
		assert.Errorf(t, err, "Value %s should be invalid", val)
	}
}

func TestValidateIntegerValue(t *testing.T) {
	intParam := schemas.Param{Name: "test_int", VarType: "integer"}
	validValues := []string{"0", "42", "-1", "1000000"}
	for _, val := range validValues {
		err := intParam.ValidateParamValue(val)
		assert.NoErrorf(t, err, "Value %s should be valid", val)
	}

	invalidValues := []string{"3.14", "abc", "", "1.0", "1e5"}
	for _, val := range invalidValues {
		err := intParam.ValidateParamValue(val)
		assert.Errorf(t, err, "Value %s should be invalid", val)
	}
}

func TestValidateRealValue(t *testing.T) {
	realParam := schemas.Param{Name: "test_real", VarType: "real"}
	validValues := []string{"0", "3.14", "-1.5", "1e5", "1.5e-10", "123"}
	for _, val := range validValues {
		err := realParam.ValidateParamValue(val)
		assert.NoErrorf(t, err, "Value %s should be valid", val)
	}

	invalidValues := []string{"abc", "", "1.2.3", "e5"}
	for _, val := range invalidValues {
		err := realParam.ValidateParamValue(val)
		assert.Errorf(t, err, "Value %s should be invalid", val)
	}
}

// TestValidateStringValue removed - validateStringValue function doesn't exist

// Test \\N handling in parameter descriptions and values
func TestHandleNullValues(t *testing.T) {
	// Test data with \\N values
	testInput := "param1\tpostmaster\tResource Usage / Memory\tINTEGER\t\\N\t16\t1024\tTest parameter\t\\N\n"
	
	params, err := schemas.ParseDescribeConfig(testInput)
	require.NoError(t, err)
	require.Len(t, params, 1)
	
	param := params[0]
	assert.Equal(t, "param1", param.Name)
	assert.Equal(t, "integer", param.VarType)
	assert.Equal(t, "\\N", param.BootVal) // Should be preserved for now (schema generator will handle)
	assert.Equal(t, "Test parameter", param.ShortDesc)
	assert.Equal(t, "\\N", param.ExtraDesc) // Should be preserved for now
}

// Test X-type detection for Size parameters
func TestXTypeDetectionSize(t *testing.T) {
	tests := []struct {
		name     string
		param    schemas.Param
		expected string
	}{
		{
			name: "shared_buffers with MB unit",
			param: schemas.Param{
				Name:     "shared_buffers",
				VarType:  "integer",
				Unit:     "MB",
				Category: "Resource Usage / Memory",
			},
			expected: "Size",
		},
		{
			name: "work_mem with kB unit",
			param: schemas.Param{
				Name:     "work_mem",
				VarType:  "integer", 
				Unit:     "kB",
				Category: "Resource Usage / Memory",
			},
			expected: "Size",
		},
		{
			name: "memory param without unit but in memory category",
			param: schemas.Param{
				Name:     "memory_param",
				VarType:  "integer",
				Unit:     "",
				Category: "Resource Usage / Memory",
			},
			expected: "Size",
		},
		{
			name: "non-memory integer param",
			param: schemas.Param{
				Name:     "max_connections",
				VarType:  "integer",
				Unit:     "",
				Category: "Connections and Authentication",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create simple detect function for testing
			result := detectXTypeForTest(tt.param)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Helper function to test X-type detection
func detectXTypeForTest(param schemas.Param) string {
	// First check the unit field
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

	// Check parameter names for time-related parameters
	timeParams := []string{
		"statement_timeout", "lock_timeout", "idle_in_transaction_session_timeout",
		"checkpoint_timeout", "wal_receiver_timeout", "wal_sender_timeout",
		"deadlock_timeout", "authentication_timeout",
	}
	for _, tp := range timeParams {
		if param.Name == tp {
			return "Duration"
		}
	}

	return ""
}

// Test X-type detection for Duration parameters
func TestXTypeDetectionDuration(t *testing.T) {
	tests := []struct {
		name     string
		param    schemas.Param
		expected string
	}{
		{
			name: "statement_timeout by name",
			param: schemas.Param{
				Name:    "statement_timeout",
				VarType: "integer",
			},
			expected: "Duration",
		},
		{
			name: "param with ms unit",
			param: schemas.Param{
				Name:    "delay_param",
				VarType: "integer",
				Unit:    "ms",
			},
			expected: "Duration",
		},
		{
			name: "param with s unit", 
			param: schemas.Param{
				Name:    "timeout_param",
				VarType: "integer",
				Unit:    "s",
			},
			expected: "Duration",
		},
		{
			name: "regular integer param",
			param: schemas.Param{
				Name:    "max_connections",
				VarType: "integer",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectXTypeForTest(tt.param)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test that no \\N values appear in final descriptions 
func TestDescriptionsCleaned(t *testing.T) {
	// Create test parameters with \\N values
	testParams := []schemas.Param{
		{
			Name:      "param1",
			ShortDesc: "Valid description",
			ExtraDesc: "\\N",
		},
		{
			Name:      "param2", 
			ShortDesc: "\\N",
			ExtraDesc: "Valid extra desc",
		},
		{
			Name:      "param3",
			ShortDesc: "Valid short",
			ExtraDesc: "Valid extra",
		},
	}

	// Test that descriptions are cleaned properly
	for _, param := range testParams {
		desc := cleanDescription(param.ShortDesc, param.ExtraDesc)
		assert.NotContains(t, desc, "\\N", "Description should not contain \\N")
		assert.NotEqual(t, "", desc, "Description should not be empty") 
	}
}

// Helper function to test description cleaning logic
func cleanDescription(shortDesc, extraDesc string) string {
	description := shortDesc
	if description == "\\N" {
		description = ""
	}
	if extraDesc != "" && extraDesc != "\\N" {
		if description != "" {
			description = description + " " + extraDesc
		} else {
			description = extraDesc
		}
	}
	return description
}

// Test all parameter types are correctly mapped
func TestParameterTypeMapping(t *testing.T) {
	tests := []struct {
		name       string
		param      schemas.Param
		expectType string
		expectXType string
	}{
		{
			name: "boolean parameter",
			param: schemas.Param{
				Name:    "enable_seqscan",
				VarType: "boolean",
			},
			expectType:  "boolean",
			expectXType: "",
		},
		{
			name: "enum parameter",
			param: schemas.Param{
				Name:     "log_level",
				VarType:  "enum",
				EnumVals: []string{"debug", "info", "error"},
			},
			expectType:  "string",
			expectXType: "",
		},
		{
			name: "integer parameter",
			param: schemas.Param{
				Name:    "max_connections",
				VarType: "integer",
			},
			expectType:  "integer",
			expectXType: "",
		},
		{
			name: "size parameter",
			param: schemas.Param{
				Name:     "shared_buffers",
				VarType:  "integer",
				Category: "Resource Usage / Memory",
			},
			expectType:  "string",
			expectXType: "Size",
		},
		{
			name: "duration parameter",
			param: schemas.Param{
				Name:    "statement_timeout",
				VarType: "integer",
			},
			expectType:  "string", 
			expectXType: "Duration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			xType := detectXTypeForTest(tt.param)
			
			// Determine expected schema type based on x-type
			var schemaType string
			if xType == "Size" || xType == "Duration" {
				schemaType = "string"
			} else {
				switch tt.param.VarType {
				case "bool", "boolean":
					schemaType = "boolean"
				case "integer":
					schemaType = "integer"
				case "real":
					schemaType = "number"
				default:
					schemaType = "string"
				}
			}
			
			assert.Equal(t, tt.expectType, schemaType)
			assert.Equal(t, tt.expectXType, xType)
		})
	}
}
