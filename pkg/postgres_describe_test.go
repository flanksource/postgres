package pkg

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sample postgres --describe-config output for testing
var sampleDescribeConfigOutput string

func init() {
	data, err := os.ReadFile("testdata/postgres-describe-v17.txt")
	if err != nil {
		panic("failed to read sample describe-config output: " + err.Error())
	}
	sampleDescribeConfigOutput = string(data)
}

func TestParseDescribeConfig(t *testing.T) {
	params, err := ParseDescribeConfig(sampleDescribeConfigOutput)
	require.NoError(t, err)

	// Should parse all 6 parameters
	assert.Len(t, params, 6)

	// Test integer parameter (shared_buffers)
	sharedBuffers := GetParamByName(params, "shared_buffers")
	require.NotNil(t, sharedBuffers)
	assert.Equal(t, "shared_buffers", sharedBuffers.Name)
	assert.Equal(t, "integer", sharedBuffers.VarType)
	assert.Equal(t, "MB", sharedBuffers.Unit)
	assert.Equal(t, "Resource Usage / Memory", sharedBuffers.Category)
	assert.Equal(t, "Sets the number of shared memory buffers used by the server.", sharedBuffers.ShortDesc)
	assert.Equal(t, "", sharedBuffers.ExtraDesc)
	assert.Equal(t, "postmaster", sharedBuffers.Context)
	assert.Equal(t, "configuration", sharedBuffers.VarClass)
	assert.Equal(t, "16", sharedBuffers.MinVal)
	assert.Equal(t, "1073741823", sharedBuffers.MaxVal)
	assert.Empty(t, sharedBuffers.EnumVals)
	assert.Equal(t, "128MB", sharedBuffers.BootVal)	assert.Equal(t, float64(16), sharedBuffers.

	// Test enum parameter (log_min_messages)
	logMinMessages := GetParamByName(params, "log_min_messages")
	require.NotNil(t, logMinMessages)
	assert.Equal(t, "log_min_messages", logMinMessages.Name)
	assert.Equal(t, "enum", logMinMessages.VarType)
	assert.Equal(t, "", logMinMessages.Unit)
	assert.Equal(t, "Reporting and Logging / When to Log", logMinMessages.Category)
	assert.Equal(t, "Sets the message levels that are logged.", logMinMessages.ShortDesc)
	assert.Contains(t, logMinMessages.ExtraDesc, "Valid values are DEBUG5")
	assert.Equal(t, "sighup", logMinMessages.Context)
	assert.Equal(t, "configuration", logMinMessages.VarClass)
	assert.Equal(t, "", logMinMessages.MinVal)
	assert.Equal(t, "", logMinMessages.MaxVal)
	expectedEnums := []string{"debug5", "debug4", "debug3", "debug2", "debug1", "info", "notice", "warning", "error", "log", "fatal", "panic"}
	assert.Equal(t, expectedEnums, logMinMessages.EnumVals)
	assert.Equal(t, "warning", logMinMessages.BootVal)

	// Test boolean parameter (enable_seqscan)
	enableSeqscan := GetParamByName(params, "enable_seqscan")
	require.NotNil(t, enableSeqscan)
	assert.Equal(t, "enable_seqscan", enableSeqscan.Name)
	assert.Equal(t, "bool", enableSeqscan.VarType)
	assert.Equal(t, "", enableSeqscan.Unit)
	assert.Equal(t, "Query Tuning / Planner Method Configuration", enableSeqscan.Category)
	assert.Equal(t, "Enables the planner's use of sequential-scan plans.", enableSeqscan.ShortDesc)
	assert.Equal(t, "", enableSeqscan.ExtraDesc)
	assert.Equal(t, "user", enableSeqscan.Context)
	assert.Equal(t, "configuration", enableSeqscan.VarClass)
	expectedBoolEnums := []string{"on", "off"}
	assert.Equal(t, expectedBoolEnums, enableSeqscan.EnumVals)
	assert.Equal(t, "on", enableSeqscan.BootVal)

	// Test string parameter (shared_preload_libraries)
	sharedPreload := GetParamByName(params, "shared_preload_libraries")
	require.NotNil(t, sharedPreload)
	assert.Equal(t, "shared_preload_libraries", sharedPreload.Name)
	assert.Equal(t, "string", sharedPreload.VarType)
	assert.Equal(t, "", sharedPreload.Unit)
	assert.Equal(t, "Client Connection Defaults / Shared Library Preloading", sharedPreload.Category)
	assert.Equal(t, "Lists shared libraries to preload into server.", sharedPreload.ShortDesc)
	assert.Equal(t, "", sharedPreload.ExtraDesc)
	assert.Equal(t, "postmaster", sharedPreload.Context)
	assert.Equal(t, "configuration", sharedPreload.VarClass)
	assert.Equal(t, "", sharedPreload.MinVal)
	assert.Equal(t, "", sharedPreload.MaxVal)
	assert.Empty(t, sharedPreload.EnumVals)
	assert.Equal(t, "", sharedPreload.BootVal)
}

func TestParseEnumValues(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    "debug5,debug4,debug3,debug2,debug1,info,notice,warning,error,log,fatal,panic",
			expected: []string{"debug5", "debug4", "debug3", "debug2", "debug1", "info", "notice", "warning", "error", "log", "fatal", "panic"},
		},
		{
			input:    "on,off",
			expected: []string{"on", "off"},
		},
		{
			input:    "minimal,replica,logical",
			expected: []string{"minimal", "replica", "logical"},
		},
		{
			input:    "",
			expected: nil,
		},
		{
			input:    "-",
			expected: nil,
		},
		{
			input:    "single_value",
			expected: []string{"single_value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseEnumValues(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetParamByName(t *testing.T) {
	params := []Param{
		{Name: "param1", VarType: "string"},
		{Name: "param2", VarType: "integer"},
		{Name: "param3", VarType: "bool"},
	}

	// Test existing parameter
	result := GetParamByName(params, "param2")
	require.NotNil(t, result)
	assert.Equal(t, "param2", result.Name)
	assert.Equal(t, "integer", result.VarType)

	// Test non-existing parameter
	result = GetParamByName(params, "nonexistent")
	assert.Nil(t, result)
}

func TestFilterParamsByCategory(t *testing.T) {
	params := []Param{
		{Name: "param1", Category: "Resource Usage / Memory"},
		{Name: "param2", Category: "Resource Usage / Disk"},
		{Name: "param3", Category: "Write-Ahead Log / Settings"},
		{Name: "param4", Category: "Resource Usage / Memory"},
	}

	// Test filtering by exact category
	filtered := FilterParamsByCategory(params, "Resource Usage / Memory")
	assert.Len(t, filtered, 2)
	assert.Equal(t, "param1", filtered[0].Name)
	assert.Equal(t, "param4", filtered[1].Name)

	// Test filtering by partial category
	filtered = FilterParamsByCategory(params, "Resource Usage")
	assert.Len(t, filtered, 3)

	// Test filtering by non-existing category
	filtered = FilterParamsByCategory(params, "NonExistent")
	assert.Len(t, filtered, 0)
}

func TestFilterParamsByContext(t *testing.T) {
	params := []Param{
		{Name: "param1", Context: "postmaster"},
		{Name: "param2", Context: "sighup"},
		{Name: "param3", Context: "user"},
		{Name: "param4", Context: "postmaster"},
	}

	// Test filtering by context
	filtered := FilterParamsByContext(params, "postmaster")
	assert.Len(t, filtered, 2)
	assert.Equal(t, "param1", filtered[0].Name)
	assert.Equal(t, "param4", filtered[1].Name)

	// Test filtering by non-existing context
	filtered = FilterParamsByContext(params, "nonexistent")
	assert.Len(t, filtered, 0)
}

func TestParamValidateParamValue(t *testing.T) {
	tests := []struct {
		name     string
		param    Param
		value    string
		wantErr  bool
		errMatch string
	}{
		// Boolean parameter tests
		{
			name:    "bool valid on",
			param:   Param{Name: "test_bool", VarType: "bool"},
			value:   "on",
			wantErr: false,
		},
		{
			name:    "bool valid true",
			param:   Param{Name: "test_bool", VarType: "bool"},
			value:   "true",
			wantErr: false,
		},
		{
			name:    "bool valid 1",
			param:   Param{Name: "test_bool", VarType: "bool"},
			value:   "1",
			wantErr: false,
		},
		{
			name:     "bool invalid value",
			param:    Param{Name: "test_bool", VarType: "bool"},
			value:    "maybe",
			wantErr:  true,
			errMatch: "invalid boolean value",
		},
		// Enum parameter tests
		{
			name:    "enum valid value",
			param:   Param{Name: "test_enum", VarType: "enum", EnumVals: []string{"debug", "info", "warning"}},
			value:   "info",
			wantErr: false,
		},
		{
			name:     "enum invalid value",
			param:    Param{Name: "test_enum", VarType: "enum", EnumVals: []string{"debug", "info", "warning"}},
			value:    "trace",
			wantErr:  true,
			errMatch: "invalid enum value",
		},
		// Integer parameter tests
		{
			name:    "integer valid value",
			param:   Param{Name: "test_int", VarType: "integer"},
			value:   "42",
			wantErr: false,
		},
		{
			name:    "integer valid negative",
			param:   Param{Name: "test_int", VarType: "integer"},
			value:   "-1",
			wantErr: false,
		},
		{
			name:     "integer invalid format",
			param:    Param{Name: "test_int", VarType: "integer"},
			value:    "not_a_number",
			wantErr:  true,
			errMatch: "invalid integer format",
		},
		// Real parameter tests
		{
			name:    "real valid value",
			param:   Param{Name: "test_real", VarType: "real"},
			value:   "3.14",
			wantErr: false,
		},
		{
			name:    "real valid scientific",
			param:   Param{Name: "test_real", VarType: "real"},
			value:   "1.5e-10",
			wantErr: false,
		},
		{
			name:     "real invalid format",
			param:    Param{Name: "test_real", VarType: "real"},
			value:    "not_a_float",
			wantErr:  true,
			errMatch: "invalid real format",
		},
		// String parameter tests
		{
			name:    "string valid value",
			param:   Param{Name: "test_string", VarType: "string"},
			value:   "any string value",
			wantErr: false,
		},
		// Unknown parameter type test
		{
			name:     "unknown type",
			param:    Param{Name: "test_unknown", VarType: "unknown"},
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
	pg, err := NewEmbeddedPostgres("16.1.0")
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
		param := GetParamByName(params, paramName)
		assert.NotNilf(t, param, "Parameter %s should exist", paramName)
		if param != nil {
			assert.NotEmptyf(t, param.ShortDesc, "Parameter %s should have a description", paramName)
			assert.NotEmptyf(t, param.VarType, "Parameter %s should have a type", paramName)
		}
	}

	// Verify specific parameter types
	sharedBuffers := GetParamByName(params, "shared_buffers")
	if sharedBuffers != nil {
		assert.Equal(t, "integer", sharedBuffers.VarType)
		assert.NotEmpty(t, sharedBuffers.Unit) // Should have a unit like "8kB"
	}

	logLevel := GetParamByName(params, "log_min_messages")
	if logLevel != nil {
		assert.Equal(t, "enum", logLevel.VarType)
		assert.Greater(t, len(logLevel.EnumVals), 5) // Should have multiple log levels
	}
}

func TestParseDescribeConfigErrorHandling(t *testing.T) {
	// Test empty input
	params, err := ParseDescribeConfig("")
	assert.NoError(t, err)
	assert.Empty(t, params)

	// Test malformed line (too few fields - less than 8)
	malformedOutput := "param1\tinteger\t8kB\tcategory\tshort"
	_, err = ParseDescribeConfig(malformedOutput)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected at least 8 fields")

	// Test with whitespace lines
	outputWithWhitespace := "param1\tstring\t\tcategory\tshort\textra\tcontext\tclass\t\t\t\tdefault\n\n  \n"
	params, err = ParseDescribeConfig(outputWithWhitespace)
	assert.NoError(t, err)
	assert.Len(t, params, 1)
	assert.Equal(t, "param1", params[0].Name)
}

func TestValidateBoolValue(t *testing.T) {
	validValues := []string{"on", "off", "true", "false", "yes", "no", "1", "0", "ON", "OFF", "True", "False"}
	for _, val := range validValues {
		err := validateBoolValue(val)
		assert.NoErrorf(t, err, "Value %s should be valid", val)
	}

	invalidValues := []string{"maybe", "2", "enable", "disable", ""}
	for _, val := range invalidValues {
		err := validateBoolValue(val)
		assert.Errorf(t, err, "Value %s should be invalid", val)
	}
}

func TestValidateEnumValue(t *testing.T) {
	enumVals := []string{"debug", "info", "warning", "error"}

	// Valid values
	for _, val := range enumVals {
		err := validateEnumValue(val, enumVals)
		assert.NoErrorf(t, err, "Value %s should be valid", val)
	}

	// Invalid values
	invalidValues := []string{"trace", "verbose", "", "DEBUG"}
	for _, val := range invalidValues {
		err := validateEnumValue(val, enumVals)
		assert.Errorf(t, err, "Value %s should be invalid", val)
	}
}

func TestValidateIntegerValue(t *testing.T) {
	validValues := []string{"0", "42", "-1", "1000000"}
	for _, val := range validValues {
		err := validateIntegerValue(val, 0, 0)
		assert.NoErrorf(t, err, "Value %s should be valid", val)
	}

	invalidValues := []string{"3.14", "abc", "", "1.0", "1e5"}
	for _, val := range invalidValues {
		err := validateIntegerValue(val, 0, 0)
		assert.Errorf(t, err, "Value %s should be invalid", val)
	}
}

func TestValidateRealValue(t *testing.T) {
	validValues := []string{"0", "3.14", "-1.5", "1e5", "1.5e-10", "123"}
	for _, val := range validValues {
		err := validateRealValue(val, 0, 0)
		assert.NoErrorf(t, err, "Value %s should be valid", val)
	}

	invalidValues := []string{"abc", "", "1.2.3", "e5"}
	for _, val := range invalidValues {
		err := validateRealValue(val, 0, 0)
		assert.Errorf(t, err, "Value %s should be invalid", val)
	}
}

// TestValidateStringValue removed - validateStringValue function doesn't exist
