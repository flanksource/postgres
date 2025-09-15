package pkg

import (
	"testing"
)

func TestValidateBoolean(t *testing.T) {
	v := &Validator{}

	tests := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		// Standard PostgreSQL boolean values
		{"on", "on", false},
		{"off", "off", false},
		{"ON", "on", false},
		{"OFF", "off", false},

		// Alternative boolean representations
		{"true", "on", false},
		{"false", "off", false},
		{"TRUE", "on", false},
		{"FALSE", "off", false},
		{"t", "on", false},
		{"f", "off", false},
		{"yes", "on", false},
		{"no", "off", false},
		{"y", "on", false},
		{"n", "off", false},
		{"1", "on", false},
		{"0", "off", false},

		// Invalid values
		{"invalid", "", true},
		{"2", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := v.validateBoolean(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateBoolean(%s) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("validateBoolean(%s) = %s, want %s", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseInteger(t *testing.T) {
	v := &Validator{}

	tests := []struct {
		input    string
		expected int64
		wantErr  bool
	}{
		// Decimal values
		{"100", 100, false},
		{"0", 0, false},
		{"-100", -100, false},
		{"+100", 100, false},

		// Hexadecimal values
		{"0xFF", 255, false},
		{"0xff", 255, false},
		{"0xFFFF", 65535, false},
		{"-0xA", -10, false},

		// Octal values
		{"0377", 255, false},
		{"0640", 416, false},
		{"0777", 511, false},
		{"-0100", -64, false},

		// Invalid values
		{"0xFFPF", 0, true},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := v.parseInteger(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseInteger(%s) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("parseInteger(%s) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsCustomOption(t *testing.T) {
	v := &Validator{}

	tests := []struct {
		input    string
		expected bool
	}{
		{"pg_stat_statements.track", true},
		{"auto_explain.log_min_duration", true},
		{"custom.option", true},
		{"extension.parameter.nested", true},

		// Invalid custom options
		{"regular_parameter", false},
		{".starts_with_dot", false},
		{"has.invalid-chars", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := v.isCustomOption(tt.input)
			if got != tt.expected {
				t.Errorf("isCustomOption(%s) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsOctalParam(t *testing.T) {
	v := &Validator{}

	tests := []struct {
		input    string
		expected bool
	}{
		{"unix_socket_permissions", true},
		{"log_file_mode", true},
		{"shared_buffers", false},
		{"max_connections", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := v.isOctalParam(tt.input)
			if got != tt.expected {
				t.Errorf("isOctalParam(%s) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestValidateIntegerWithOctalFormatting(t *testing.T) {
	v := &Validator{}

	tests := []struct {
		paramName string
		input     string
		expected  string
		wantErr   bool
	}{
		// unix_socket_permissions (should format as octal)
		{"unix_socket_permissions", "511", "0777", false},
		{"unix_socket_permissions", "0640", "0640", false},
		{"unix_socket_permissions", "416", "0640", false},
		{"unix_socket_permissions", "0", "0000", false},

		// Regular integer parameter (should format as decimal)
		{"max_connections", "100", "100", false},
		{"max_connections", "0xFF", "255", false},
		{"max_connections", "0377", "255", false},
	}

	for _, tt := range tests {
		t.Run(tt.paramName+"_"+tt.input, func(t *testing.T) {
			param := &Param{
				Name:    tt.paramName,
				VarType: "integer",
			}
			got, err := v.validateInteger(param, tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateInteger(%s, %s) error = %v, wantErr %v", tt.paramName, tt.input, err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("validateInteger(%s, %s) = %s, want %s", tt.paramName, tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseMemoryValue(t *testing.T) {
	v := &Validator{}

	tests := []struct {
		input        string
		expectedVal  int64
		expectedUnit string
		wantErr      bool
	}{
		// Simple values with units
		{"128MB", 128, "MB", false},
		{"1GB", 1, "GB", false},
		{"512KB", 512, "KB", false},
		{"2TB", 2, "TB", false},

		// Values without units (default to kB)
		{"1024", 1024, "kB", false},

		// Values with spaces
		{"128 MB", 128, "MB", false},
		{"1 GB", 1, "GB", false},

		// Case variations
		{"128mb", 128, "MB", false},
		{"1gb", 1, "GB", false},
		{"512kb", 512, "KB", false},

		// Invalid values
		{"invalid", 0, "", true},
		{"", 0, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			val, unit, err := v.ParseMemoryValue(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMemoryValue(%s) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if val != tt.expectedVal {
					t.Errorf("ParseMemoryValue(%s) value = %d, want %d", tt.input, val, tt.expectedVal)
				}
				if unit != tt.expectedUnit {
					t.Errorf("ParseMemoryValue(%s) unit = %s, want %s", tt.input, unit, tt.expectedUnit)
				}
			}
		})
	}
}

func TestNormalizeMemory(t *testing.T) {
	v := &Validator{}

	tests := []struct {
		bytes        int64
		expectedVal  int64
		expectedUnit string
	}{
		// Exact conversions
		{1024, 1, "KB"},
		{1024 * 1024, 1, "MB"},
		{1024 * 1024 * 1024, 1, "GB"},
		{1024 * 1024 * 1024 * 1024, 1, "TB"},

		// Should stay in bytes if not exact
		{1023, 1023, "B"},
		{1025, 1025, "B"},

		// Larger values
		{2 * 1024 * 1024, 2, "MB"},
		{512 * 1024, 512, "KB"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			val, unit := v.normalizeMemory(tt.bytes)
			if val != tt.expectedVal {
				t.Errorf("normalizeMemory(%d) value = %d, want %d", tt.bytes, val, tt.expectedVal)
			}
			if unit != tt.expectedUnit {
				t.Errorf("normalizeMemory(%d) unit = %s, want %s", tt.bytes, unit, tt.expectedUnit)
			}
		})
	}
}

func TestValidateEnum(t *testing.T) {
	v := &Validator{}

	param := &Param{
		Name:     "log_statement",
		VarType:  "enum",
		EnumVals: []string{"none", "ddl", "mod", "all"},
	}

	tests := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		// Valid values (case-insensitive)
		{"none", "none", false},
		{"ddl", "ddl", false},
		{"mod", "mod", false},
		{"all", "all", false},
		{"NONE", "none", false},
		{"DDL", "ddl", false},
		{"All", "all", false},

		// Invalid values
		{"invalid", "", true},
		{"", "", true},
		{"partial", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := v.validateEnum(param, tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEnum(%s) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("validateEnum(%s) = %s, want %s", tt.input, got, tt.expected)
			}
		})
	}
}
