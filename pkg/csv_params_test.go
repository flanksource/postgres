package pkg

import (
	"testing"
)

func TestGetPgVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    PgVersion
		wantErr bool
	}{
		{"PostgreSQL 9.1", "9.1.24", PgVersion9_1, false},
		{"PostgreSQL 9.6", "9.6.20", PgVersion9_6, false},
		{"PostgreSQL 10", "10.15", PgVersion10, false},
		{"PostgreSQL 11", "11.10", PgVersion11, false},
		{"PostgreSQL 12", "12.5", PgVersion12, false},
		{"PostgreSQL 13", "13.1", PgVersion13, false},
		{"PostgreSQL 14", "14.0", PgVersion14, false},
		{"PostgreSQL 15", "15.2", PgVersion15, false},
		{"PostgreSQL 16", "16.1", PgVersion16, false},
		{"PostgreSQL 17", "17.0", PgVersion17, false},
		{"PostgreSQL 18", "18.0", PgVersion18, false},
		{"Future version", "19.0", PgVersion18, false}, // Defaults to latest
		{"Invalid version", "invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetPgVersion(tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPgVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetPgVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseCSVParams(t *testing.T) {
	csvContent := `shared_buffers	integer	128MB	MB	16	1073741823	\N
max_connections	integer	100	\N	1	262143	\N
enable_seqscan	bool	on	\N	\N	\N	\N
log_statement	enum	none	\N	\N	\N	{none,ddl,mod,all}
work_mem	integer	4MB	kB	64	2147483647	\N`

	params, err := ParseCSVParams(csvContent)
	if err != nil {
		t.Fatalf("ParseCSVParams() error = %v", err)
	}

	if len(params) != 5 {
		t.Errorf("Expected 5 parameters, got %d", len(params))
	}

	// Test shared_buffers
	if params[0].Name != "shared_buffers" {
		t.Errorf("Expected name 'shared_buffers', got '%s'", params[0].Name)
	}
	if params[0].Type != "integer" {
		t.Errorf("Expected type 'integer', got '%s'", params[0].Type)
	}
	if params[0].DefaultValue != "128MB" {
		t.Errorf("Expected default '128MB', got '%s'", params[0].DefaultValue)
	}
	if params[0].Unit != "MB" {
		t.Errorf("Expected unit 'MB', got '%s'", params[0].Unit)
	}

	// Test enable_seqscan
	if params[2].Name != "enable_seqscan" {
		t.Errorf("Expected name 'enable_seqscan', got '%s'", params[2].Name)
	}
	if params[2].Type != "bool" {
		t.Errorf("Expected type 'bool', got '%s'", params[2].Type)
	}

	// Test log_statement enum values
	if params[3].Name != "log_statement" {
		t.Errorf("Expected name 'log_statement', got '%s'", params[3].Name)
	}
	if len(params[3].EnumValues) != 4 {
		t.Errorf("Expected 4 enum values, got %d", len(params[3].EnumValues))
	}
	expectedEnums := []string{"none", "ddl", "mod", "all"}
	for i, expected := range expectedEnums {
		if params[3].EnumValues[i] != expected {
			t.Errorf("Expected enum value '%s', got '%s'", expected, params[3].EnumValues[i])
		}
	}
}

func TestFromNull(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"\\N", ""},
		{"value", "value"},
		{"", ""},
		{"0", "0"},
	}

	for _, tt := range tests {
		result := fromNull(tt.input)
		if result != tt.expected {
			t.Errorf("fromNull(%s) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestParseCSVEnumValues(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"{none,ddl,mod,all}", []string{"none", "ddl", "mod", "all"}},
		{"{on,off}", []string{"on", "off"}},
		{"\\N", nil},
		{"", nil},
		{"{}", nil},
		{"{single}", []string{"single"}},
		{`{"quoted value","another"}`, []string{"quoted value", "another"}},
	}

	for _, tt := range tests {
		result := parseCSVEnumValues(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("parseCSVEnumValues(%s) length = %d, want %d", tt.input, len(result), len(tt.expected))
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("parseCSVEnumValues(%s)[%d] = %s, want %s", tt.input, i, result[i], tt.expected[i])
			}
		}
	}
}

func TestConvertCSVToParam(t *testing.T) {
	csv := CSVParam{
		Name:         "shared_buffers",
		Type:         "integer",
		DefaultValue: "128MB",
		Unit:         "MB",
		MinValue:     "16",
		MaxValue:     "1073741823",
		EnumValues:   nil,
	}

	param := ConvertCSVToParam(csv)

	if param.Name != "shared_buffers" {
		t.Errorf("Expected name 'shared_buffers', got '%s'", param.Name)
	}
	if param.VarType != "integer" {
		t.Errorf("Expected type 'integer', got '%s'", param.VarType)
	}
	if param.BootVal != "128MB" {
		t.Errorf("Expected boot value '128MB', got '%s'", param.BootVal)
	}
	if param.MinVal != 16 {
		t.Errorf("Expected min value 16, got %f", param.MinVal)
	}
	if param.MaxVal != 1073741823 {
		t.Errorf("Expected max value 1073741823, got %f", param.MaxVal)
	}
}
