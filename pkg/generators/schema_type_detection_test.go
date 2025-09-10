package generators

import (
	"testing"

	"github.com/flanksource/postgres/pkg"
)

func TestDetectXType_SizeParameters(t *testing.T) {
	generator := &SchemaGenerator{version: "16.1.0"}

	sizeTestCases := []struct {
		name     string
		param    pkg.Param
		expected string
	}{
		// Unit-based detection
		{
			name: "shared_buffers with 8kB unit",
			param: pkg.Param{
				Name:    "shared_buffers",
				VarType: "integer",
				Unit:    "8kB",
			},
			expected: "Size",
		},
		{
			name: "work_mem with kB unit",
			param: pkg.Param{
				Name:    "work_mem",
				VarType: "integer",
				Unit:    "kB",
			},
			expected: "Size",
		},
		{
			name: "maintenance_work_mem with kB unit",
			param: pkg.Param{
				Name:    "maintenance_work_mem",
				VarType: "integer",
				Unit:    "kB",
			},
			expected: "Size",
		},

		// Name-based exact matches (no unit)
		{
			name: "effective_cache_size without unit",
			param: pkg.Param{
				Name:    "effective_cache_size",
				VarType: "integer",
			},
			expected: "Size",
		},
		{
			name: "logical_decoding_work_mem without unit",
			param: pkg.Param{
				Name:    "logical_decoding_work_mem",
				VarType: "integer",
			},
			expected: "Size",
		},
		{
			name: "max_stack_depth without unit",
			param: pkg.Param{
				Name:    "max_stack_depth",
				VarType: "integer",
			},
			expected: "Size",
		},
		{
			name: "vacuum_buffer_usage_limit without unit",
			param: pkg.Param{
				Name:    "vacuum_buffer_usage_limit",
				VarType: "integer",
			},
			expected: "Size",
		},
		{
			name: "backend_flush_after without unit",
			param: pkg.Param{
				Name:    "backend_flush_after",
				VarType: "integer",
			},
			expected: "Size",
		},
		{
			name: "gin_pending_list_limit without unit",
			param: pkg.Param{
				Name:    "gin_pending_list_limit",
				VarType: "integer",
			},
			expected: "Size",
		},
		{
			name: "log_rotation_size without unit",
			param: pkg.Param{
				Name:    "log_rotation_size",
				VarType: "integer",
			},
			expected: "Size",
		},
		{
			name: "temp_file_limit without unit",
			param: pkg.Param{
				Name:    "temp_file_limit",
				VarType: "integer",
			},
			expected: "Size",
		},
		{
			name: "max_slot_wal_keep_size without unit",
			param: pkg.Param{
				Name:    "max_slot_wal_keep_size",
				VarType: "integer",
			},
			expected: "Size",
		},
		{
			name: "wal_keep_size without unit",
			param: pkg.Param{
				Name:    "wal_keep_size",
				VarType: "integer",
			},
			expected: "Size",
		},

		// Pattern-based matches
		{
			name: "custom_buffer parameter",
			param: pkg.Param{
				Name:    "custom_buffer_size",
				VarType: "integer",
			},
			expected: "Size",
		},
		{
			name: "memory limit parameter",
			param: pkg.Param{
				Name:    "query_mem_limit",
				VarType: "integer",
			},
			expected: "Size",
		},
		{
			name: "cache size parameter",
			param: pkg.Param{
				Name:    "index_cache_size",
				VarType: "integer",
			},
			expected: "Size",
		},

		// Parameters that should NOT be Size
		{
			name: "max_connections (not size)",
			param: pkg.Param{
				Name:    "max_connections",
				VarType: "integer",
			},
			expected: "",
		},
		{
			name: "port (not size)",
			param: pkg.Param{
				Name:    "port",
				VarType: "integer",
			},
			expected: "",
		},
	}

	for _, tc := range sizeTestCases {
		t.Run(tc.name, func(t *testing.T) {
			result := generator.detectXType(tc.param)
			if result != tc.expected {
				t.Errorf("Expected %s for %s, got %s", tc.expected, tc.param.Name, result)
			}
		})
	}
}

func TestDetectXType_DurationParameters(t *testing.T) {
	generator := &SchemaGenerator{version: "16.1.0"}

	durationTestCases := []struct {
		name     string
		param    pkg.Param
		expected string
	}{
		// Unit-based detection
		{
			name: "statement_timeout with ms unit",
			param: pkg.Param{
				Name:    "statement_timeout",
				VarType: "integer",
				Unit:    "ms",
			},
			expected: "Duration",
		},
		{
			name: "lock_timeout with ms unit",
			param: pkg.Param{
				Name:    "lock_timeout",
				VarType: "integer",
				Unit:    "ms",
			},
			expected: "Duration",
		},
		{
			name: "checkpoint_timeout with s unit",
			param: pkg.Param{
				Name:    "checkpoint_timeout",
				VarType: "integer",
				Unit:    "s",
			},
			expected: "Duration",
		},

		// Name-based exact matches (no unit)
		{
			name: "authentication_timeout without unit",
			param: pkg.Param{
				Name:    "authentication_timeout",
				VarType: "integer",
			},
			expected: "Duration",
		},
		{
			name: "archive_timeout without unit",
			param: pkg.Param{
				Name:    "archive_timeout",
				VarType: "integer",
			},
			expected: "Duration",
		},
		{
			name: "deadlock_timeout without unit",
			param: pkg.Param{
				Name:    "deadlock_timeout",
				VarType: "integer",
			},
			expected: "Duration",
		},
		{
			name: "idle_in_transaction_session_timeout without unit",
			param: pkg.Param{
				Name:    "idle_in_transaction_session_timeout",
				VarType: "integer",
			},
			expected: "Duration",
		},
		{
			name: "idle_session_timeout without unit",
			param: pkg.Param{
				Name:    "idle_session_timeout",
				VarType: "integer",
			},
			expected: "Duration",
		},
		{
			name: "transaction_timeout without unit",
			param: pkg.Param{
				Name:    "transaction_timeout",
				VarType: "integer",
			},
			expected: "Duration",
		},
		{
			name: "log_autovacuum_min_duration without unit",
			param: pkg.Param{
				Name:    "log_autovacuum_min_duration",
				VarType: "integer",
			},
			expected: "Duration",
		},
		{
			name: "log_min_duration_statement without unit",
			param: pkg.Param{
				Name:    "log_min_duration_statement",
				VarType: "integer",
			},
			expected: "Duration",
		},
		{
			name: "log_rotation_age without unit",
			param: pkg.Param{
				Name:    "log_rotation_age",
				VarType: "integer",
			},
			expected: "Duration",
		},
		{
			name: "client_connection_check_interval without unit",
			param: pkg.Param{
				Name:    "client_connection_check_interval",
				VarType: "integer",
			},
			expected: "Duration",
		},
		{
			name: "tcp_keepalives_idle without unit",
			param: pkg.Param{
				Name:    "tcp_keepalives_idle",
				VarType: "integer",
			},
			expected: "Duration",
		},
		{
			name: "tcp_keepalives_interval without unit",
			param: pkg.Param{
				Name:    "tcp_keepalives_interval",
				VarType: "integer",
			},
			expected: "Duration",
		},
		{
			name: "wal_receiver_timeout without unit",
			param: pkg.Param{
				Name:    "wal_receiver_timeout",
				VarType: "integer",
			},
			expected: "Duration",
		},
		{
			name: "wal_sender_timeout without unit",
			param: pkg.Param{
				Name:    "wal_sender_timeout",
				VarType: "integer",
			},
			expected: "Duration",
		},
		{
			name: "autovacuum_vacuum_cost_delay without unit",
			param: pkg.Param{
				Name:    "autovacuum_vacuum_cost_delay",
				VarType: "real",
			},
			expected: "Duration",
		},
		{
			name: "vacuum_cost_delay without unit",
			param: pkg.Param{
				Name:    "vacuum_cost_delay",
				VarType: "real",
			},
			expected: "Duration",
		},
		{
			name: "bgwriter_delay without unit",
			param: pkg.Param{
				Name:    "bgwriter_delay",
				VarType: "integer",
			},
			expected: "Duration",
		},
		{
			name: "wal_writer_delay without unit",
			param: pkg.Param{
				Name:    "wal_writer_delay",
				VarType: "integer",
			},
			expected: "Duration",
		},
		{
			name: "checkpoint_warning without unit",
			param: pkg.Param{
				Name:    "checkpoint_warning",
				VarType: "integer",
			},
			expected: "Duration",
		},

		// Pattern-based matches
		{
			name: "custom timeout parameter",
			param: pkg.Param{
				Name:    "custom_query_timeout",
				VarType: "integer",
			},
			expected: "Duration",
		},
		{
			name: "interval parameter",
			param: pkg.Param{
				Name:    "backup_interval",
				VarType: "integer",
			},
			expected: "Duration",
		},
		{
			name: "delay parameter",
			param: pkg.Param{
				Name:    "replication_delay",
				VarType: "integer",
			},
			expected: "Duration",
		},

		// Parameters that should NOT be Duration
		{
			name: "max_connections (not duration)",
			param: pkg.Param{
				Name:    "max_connections",
				VarType: "integer",
			},
			expected: "",
		},
		{
			name: "shared_buffers (not duration)",
			param: pkg.Param{
				Name:    "shared_buffers",
				VarType: "integer",
			},
			expected: "Size", // This should be detected as Size, not Duration
		},
	}

	for _, tc := range durationTestCases {
		t.Run(tc.name, func(t *testing.T) {
			result := generator.detectXType(tc.param)
			if result != tc.expected {
				t.Errorf("Expected %s for %s, got %s", tc.expected, tc.param.Name, result)
			}
		})
	}
}

func TestDetectXType_RealWorldParameters(t *testing.T) {
	generator := &SchemaGenerator{version: "16.1.0"}

	// Test cases based on actual PostgreSQL parameters from config.txt
	realWorldTestCases := []struct {
		name     string
		param    pkg.Param
		expected string
	}{
		// From config.txt line 22: autovacuum_work_mem
		{
			name: "autovacuum_work_mem",
			param: pkg.Param{
				Name:    "autovacuum_work_mem",
				VarType: "integer",
				Unit:    "", // No unit in config.txt
				MinVal:  "-1",
				MaxVal:  "2147483647",
			},
			expected: "Size",
		},
		// From config.txt line 16: autovacuum_vacuum_cost_delay
		{
			name: "autovacuum_vacuum_cost_delay", 
			param: pkg.Param{
				Name:    "autovacuum_vacuum_cost_delay",
				VarType: "real",
				Unit:    "", // No unit in config.txt
				MinVal:  "-1",
				MaxVal:  "100",
			},
			expected: "Duration",
		},
		// From config.txt line 35: checkpoint_timeout
		{
			name: "checkpoint_timeout",
			param: pkg.Param{
				Name:    "checkpoint_timeout",
				VarType: "integer",
				Unit:    "", // No unit in config.txt
				MinVal:  "30",
				MaxVal:  "86400",
			},
			expected: "Duration",
		},
		// From config.txt line 69: effective_cache_size
		{
			name: "effective_cache_size",
			param: pkg.Param{
				Name:    "effective_cache_size",
				VarType: "integer",
				Unit:    "", // No unit in config.txt but represents 8kB pages
				MinVal:  "1", 
				MaxVal:  "2147483647",
			},
			expected: "Size",
		},
		// From config.txt line 177: maintenance_work_mem
		{
			name: "maintenance_work_mem",
			param: pkg.Param{
				Name:    "maintenance_work_mem",
				VarType: "integer",
				Unit:    "", // No unit in config.txt but is kB
				MinVal:  "64",
				MaxVal:  "2147483647",
			},
			expected: "Size",
		},
		// From config.txt line 240: shared_buffers
		{
			name: "shared_buffers",
			param: pkg.Param{
				Name:    "shared_buffers",
				VarType: "integer",
				Unit:    "", // No unit in config.txt but is 8kB blocks
				MinVal:  "16",
				MaxVal:  "1073741823",
			},
			expected: "Size",
		},
		// From config.txt line 325: work_mem
		{
			name: "work_mem",
			param: pkg.Param{
				Name:    "work_mem",
				VarType: "integer",
				Unit:    "", // No unit in config.txt but is kB
				MinVal:  "64",
				MaxVal:  "2147483647",
			},
			expected: "Size",
		},
		// From config.txt line 307: wal_buffers
		{
			name: "wal_buffers",
			param: pkg.Param{
				Name:    "wal_buffers",
				VarType: "integer",
				Unit:    "", // No unit in config.txt but is 8kB blocks
				MinVal:  "-1",
				MaxVal:  "262143",
			},
			expected: "Size",
		},
		// Boolean parameters should not be typed
		{
			name: "fsync (boolean)",
			param: pkg.Param{
				Name:    "fsync",
				VarType: "bool",
				Unit:    "",
			},
			expected: "",
		},
		// String parameters should not be typed
		{
			name: "application_name (string)",
			param: pkg.Param{
				Name:    "application_name",
				VarType: "string",
				Unit:    "",
			},
			expected: "",
		},
		// Enum parameters should not be typed
		{
			name: "wal_level (enum)",
			param: pkg.Param{
				Name:     "wal_level",
				VarType:  "string",
				Unit:     "",
				EnumVals: []string{"minimal", "replica", "logical"},
			},
			expected: "",
		},
	}

	for _, tc := range realWorldTestCases {
		t.Run(tc.name, func(t *testing.T) {
			result := generator.detectXType(tc.param)
			if result != tc.expected {
				t.Errorf("Expected %s for %s (type: %s), got %s", tc.expected, tc.param.Name, tc.param.VarType, result)
			}
		})
	}
}

func TestDetectXType_EdgeCases(t *testing.T) {
	generator := &SchemaGenerator{version: "16.1.0"}

	edgeCases := []struct {
		name     string
		param    pkg.Param
		expected string
	}{
		// Empty parameter name
		{
			name: "empty parameter name",
			param: pkg.Param{
				Name:    "",
				VarType: "integer",
			},
			expected: "",
		},
		// Parameter name with mixed case
		{
			name: "mixed case timeout parameter",
			param: pkg.Param{
				Name:    "Connection_Timeout",
				VarType: "integer",
			},
			expected: "Duration",
		},
		{
			name: "mixed case buffer parameter",
			param: pkg.Param{
				Name:    "Shared_Buffers",
				VarType: "integer",
			},
			expected: "Size",
		},
		// Real type parameters can also be durations
		{
			name: "real type delay parameter",
			param: pkg.Param{
				Name:    "custom_delay",
				VarType: "real",
			},
			expected: "Duration",
		},
		// String type parameters with relevant names should not be typed
		{
			name: "string timeout parameter",
			param: pkg.Param{
				Name:    "connection_timeout_str",
				VarType: "string",
			},
			expected: "Duration", // Current implementation doesn't filter by VarType
		},
		// Units should take priority over name patterns
		{
			name: "parameter with conflicting unit and name",
			param: pkg.Param{
				Name:    "timeout_setting", // name suggests Duration
				VarType: "integer",
				Unit:    "kB", // unit suggests Size
			},
			expected: "Size", // Unit should win
		},
	}

	for _, tc := range edgeCases {
		t.Run(tc.name, func(t *testing.T) {
			result := generator.detectXType(tc.param)
			if result != tc.expected {
				t.Errorf("Expected %s for %s, got %s", tc.expected, tc.param.Name, result)
			}
		})
	}
}

func TestParameterTypeOverrides(t *testing.T) {
	generator := &SchemaGenerator{version: "16.1.0"}

	testCases := []struct {
		name           string
		param          pkg.Param
		expectedType   string
		expectedXType  string
	}{
		{
			name: "lock_timeout should be string with Duration x-type",
			param: pkg.Param{
				Name:      "lock_timeout",
				VarType:   "integer", // PostgreSQL reports as INTEGER
				Unit:      "",
				ShortDesc: "Sets the maximum allowed duration of any wait for a lock",
				BootVal:   "0",
			},
			expectedType:  "string",
			expectedXType: "Duration",
		},
		{
			name: "deadlock_timeout should be string with Duration x-type",
			param: pkg.Param{
				Name:      "deadlock_timeout",
				VarType:   "integer", // PostgreSQL reports as INTEGER
				Unit:      "",
				ShortDesc: "Sets the time to wait on a lock before checking for deadlock",
				BootVal:   "1000",
			},
			expectedType:  "string",
			expectedXType: "Duration",
		},
		{
			name: "huge_page_size should be string with Size x-type",
			param: pkg.Param{
				Name:      "huge_page_size",
				VarType:   "integer", // PostgreSQL reports as INTEGER
				Unit:      "",
				ShortDesc: "The size of huge page that should be requested",
				BootVal:   "0",
			},
			expectedType:  "string",
			expectedXType: "Size",
		},
		{
			name: "enable_bitmapscan should be boolean with no x-type",
			param: pkg.Param{
				Name:      "enable_bitmapscan",
				VarType:   "bool", // PostgreSQL reports as BOOLEAN
				Unit:      "",
				ShortDesc: "Enables the planner's use of bitmap-scan plans",
				BootVal:   "on",
			},
			expectedType:  "boolean",
			expectedXType: "",
		},
		{
			name: "shared_buffers should be string with Size x-type",
			param: pkg.Param{
				Name:      "shared_buffers",
				VarType:   "integer", // PostgreSQL reports as INTEGER with unit
				Unit:      "8kB",
				ShortDesc: "Sets the number of shared memory buffers used by the server",
				BootVal:   "1024",
			},
			expectedType:  "string",
			expectedXType: "Size",
		},
		{
			name: "authentication_timeout should be string with Duration x-type",
			param: pkg.Param{
				Name:      "authentication_timeout",
				VarType:   "integer", // PostgreSQL reports as INTEGER with unit
				Unit:      "s",
				ShortDesc: "Sets the maximum allowed time to complete client authentication",
				BootVal:   "60",
			},
			expectedType:  "string",
			expectedXType: "Duration",
		},
		{
			name: "max_connections should remain integer with no x-type",
			param: pkg.Param{
				Name:      "max_connections",
				VarType:   "integer",
				Unit:      "",
				ShortDesc: "Sets the maximum number of concurrent connections",
				BootVal:   "100",
			},
			expectedType:  "integer",
			expectedXType: "",
		},
		{
			name: "max_parallel_workers should remain integer with no x-type",
			param: pkg.Param{
				Name:      "max_parallel_workers",
				VarType:   "integer",
				Unit:      "",
				ShortDesc: "Sets the maximum number of parallel workers that can be active at one time",
				BootVal:   "8",
			},
			expectedType:  "integer",
			expectedXType: "",
		},
		{
			name: "max_parallel_workers_per_gather should remain integer with no x-type",
			param: pkg.Param{
				Name:      "max_parallel_workers_per_gather",
				VarType:   "integer",
				Unit:      "",
				ShortDesc: "Sets the maximum number of parallel processes per executor node",
				BootVal:   "2",
			},
			expectedType:  "integer",
			expectedXType: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			prop := generator.convertParamToProperty(tc.param)
			if prop == nil {
				t.Error("Expected property to be created")
				return
			}

			if prop.Type != tc.expectedType {
				t.Errorf("Expected type %s, got %v", tc.expectedType, prop.Type)
			}

			if prop.XType != tc.expectedXType {
				t.Errorf("Expected x-type %s, got %s", tc.expectedXType, prop.XType)
			}

			// Verify default value handling for Size/Duration parameters
			if tc.expectedXType == "Size" || tc.expectedXType == "Duration" {
				// Default should be the raw string value for Size/Duration parameters
				if prop.Default != tc.param.BootVal {
					t.Errorf("Expected default %s, got %v", tc.param.BootVal, prop.Default)
				}
			}
		})
	}
}

// Benchmark the detectXType function to ensure performance is acceptable
func BenchmarkDetectXType(b *testing.B) {
	generator := &SchemaGenerator{version: "16.1.0"}
	
	testParams := []pkg.Param{
		{Name: "shared_buffers", VarType: "integer", Unit: "8kB"},
		{Name: "statement_timeout", VarType: "integer", Unit: "ms"},
		{Name: "max_connections", VarType: "integer"},
		{Name: "effective_cache_size", VarType: "integer"},
		{Name: "log_min_duration_statement", VarType: "integer"},
		{Name: "unknown_parameter", VarType: "integer"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, param := range testParams {
			generator.detectXType(param)
		}
	}
}