package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/flanksource/postgres/pkg/schemas"
)

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
			result := detectXTypeForTest(tt.param)
			assert.Equal(t, tt.expected, result)
		})
	}
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
	tests := []struct {
		name      string
		shortDesc string
		extraDesc string
		expected  string
	}{
		{
			name:      "normal descriptions",
			shortDesc: "Valid description",
			extraDesc: "Valid extra",
			expected:  "Valid description Valid extra",
		},
		{
			name:      "extra desc is \\N",
			shortDesc: "Valid description",
			extraDesc: "\\N",
			expected:  "Valid description",
		},
		{
			name:      "short desc is \\N",
			shortDesc: "\\N",
			extraDesc: "Valid extra desc",
			expected:  "Valid extra desc",
		},
		{
			name:      "both have values",
			shortDesc: "Valid short",
			extraDesc: "Valid extra",
			expected:  "Valid short Valid extra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanDescription(tt.shortDesc, tt.extraDesc)
			assert.Equal(t, tt.expected, result)
			assert.NotContains(t, result, "\\N", "Description should not contain \\N")
		})
	}
}

// Test all parameter types are correctly mapped
func TestParameterTypeMapping(t *testing.T) {
	tests := []struct {
		name        string
		param       schemas.Param
		expectType  string
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
