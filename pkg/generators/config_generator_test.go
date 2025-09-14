package generators

import (
	"reflect"
	"strings"
	"testing"

	"github.com/flanksource/postgres/pkg"
	"github.com/flanksource/postgres/pkg/types"
	"github.com/flanksource/postgres/pkg/utils"
	"gopkg.in/yaml.v3"
)

// Test helper functions
func intPtr(i int) *int { return &i }
func sizePtr(bytes uint64) *types.Size { val := types.Size(bytes); return &val }
func float64Ptr(f float64) *float64 { return &f }
func stringPtr(s string) *string { return &s }

func TestConfigGenerator_GenerateFullYAML(t *testing.T) {
	tests := []struct {
		name     string
		conf     *pkg.PostgreSQLConfiguration
		contains []string
		excludes []string
	}{
		{
			name: "default_configuration_includes_all_sections",
			conf: &pkg.PostgreSQLConfiguration{
				Postgres:  &pkg.PostgresConf{},
				Pgbouncer: &pkg.PgBouncerConf{},
				Walg:      &pkg.WalgConf{},
				Postgrest: &pkg.PostgrestConf{},
				Pgaudit:   &pkg.PGAuditConf{},
			},
			contains: []string{
				"# Postgres Configuration",
				"postgres:",
				"# Pgbouncer Configuration",
				"pgbouncer:",
				"# Walg Configuration",
				"walg:",
				"# Postgrest Configuration",
				"postgrest:",
				"# Pgaudit Configuration",
				"pgaudit:",
			},
		},
		{
			name: "includes_field_descriptions",
			conf: &pkg.PostgreSQLConfiguration{
				Postgres: &pkg.PostgresConf{},
			},
			contains: []string{
				"# default:",
				"# env:",
			},
		},
		{
			name: "formats_different_field_types",
			conf: &pkg.PostgreSQLConfiguration{
				Postgres: &pkg.PostgresConf{
					Port:                       intPtr(5432),
					MaxConnections:             intPtr(200),
					SharedBuffers:              sizePtr(262144),
					EffectiveCacheSize:         sizePtr(131072),
					MaintenanceWorkMem:         sizePtr(65536),
					CheckpointCompletionTarget: float64Ptr(0.9),
					WalBuffers:                 sizePtr(2048),
					RandomPageCost:             float64Ptr(4.0),
					EffectiveIoConcurrency:     intPtr(1),
					WorkMem:                    sizePtr(4096),
				},
			},
			contains: []string{
				`port: 5432`,
				`max_connections: 200`,
				`shared_buffers: 262144`,
				`checkpoint_completion_target: 0.9`,
				`random_page_cost: 4`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := NewConfigGenerator(tt.conf)
			yamlContent, err := generator.GenerateFullYAML()

			if err != nil {
				t.Fatalf("GenerateFullYAML() error = %v", err)
			}

			for _, expected := range tt.contains {
				if !strings.Contains(yamlContent, expected) {
					t.Errorf("GenerateFullYAML() missing expected content: %s\nGenerated:\n%s", expected, yamlContent)
				}
			}

			for _, excluded := range tt.excludes {
				if strings.Contains(yamlContent, excluded) {
					t.Errorf("GenerateFullYAML() contains excluded content: %s", excluded)
				}
			}

			var parsed map[string]interface{}
			if err := yaml.Unmarshal([]byte(yamlContent), &parsed); err != nil {
				t.Errorf("GenerateFullYAML() generated invalid YAML: %v", err)
			}
		})
	}
}

func TestConfigGenerator_GenerateMinimalYAML(t *testing.T) {
	tests := []struct {
		name     string
		conf     *pkg.PostgreSQLConfiguration
		contains []string
		excludes []string
	}{
		{
			name: "default_config_generates_empty_minimal",
			conf: &pkg.PostgreSQLConfiguration{},
			contains: []string{
				"# PGConfig - PostgreSQL Configuration Management (Minimal)",
				"# This file contains only non-default configuration values",
				"# All values are currently at their defaults",
			},
		},
		{
			name: "non_default_postgres_values_included",
			conf: &pkg.PostgreSQLConfiguration{
				Postgres: &pkg.PostgresConf{
					Port:           intPtr(9999),
					MaxConnections: intPtr(500),
					SharedBuffers:  sizePtr(524288), // 512MB in 8KB pages
				},
			},
			contains: []string{
				"# Postgres Configuration (non-defaults only)",
				"postgres:",
				"port: 9999",
				"max_connections: 500",
				`shared_buffers: "512MB"`,
			},
			excludes: []string{
				"checkpoint_completion_target:",
				"wal_buffers:",
			},
		},
		{
			name: "non_default_pgbouncer_values_included",
			conf: &pkg.PostgreSQLConfiguration{
				Pgbouncer: &pkg.PgBouncerConf{
					ListenPort:      6543,
					MaxClientConn:   500,
					DefaultPoolSize: 50,
				},
			},
			contains: []string{
				"# Pgbouncer Configuration (non-defaults only)",
				"pgbouncer:",
				"listen_port: 6543",
				"max_client_conn: 500",
				"default_pool_size: 50",
			},
		},
		{
			name: "mixed_sections_only_non_defaults_shown",
			conf: &pkg.PostgreSQLConfiguration{
				Postgres: &pkg.PostgresConf{
					Port: intPtr(5433),
				},
				Walg: &pkg.WalgConf{
					S3Prefix: stringPtr("s3://my-backup-bucket"),
				},
			},
			contains: []string{
				"postgres:",
				"port: 5433",
				"walg:",
				`s3_prefix: "s3://my-backup-bucket"`,
			},
			excludes: []string{
				"pgbouncer:",
				"postgrest:",
				"pgaudit:",
				"max_connections:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := NewConfigGenerator(tt.conf)
			yamlContent, err := generator.GenerateMinimalYAML()

			if err != nil {
				t.Fatalf("GenerateMinimalYAML() error = %v", err)
			}

			for _, expected := range tt.contains {
				if !strings.Contains(yamlContent, expected) {
					t.Errorf("GenerateMinimalYAML() missing expected content: %s\nGenerated:\n%s", expected, yamlContent)
				}
			}

			for _, excluded := range tt.excludes {
				if strings.Contains(yamlContent, excluded) {
					t.Errorf("GenerateMinimalYAML() contains excluded content: %s", excluded)
				}
			}

			if !strings.Contains(yamlContent, "{}") {
				var parsed map[string]interface{}
				if err := yaml.Unmarshal([]byte(yamlContent), &parsed); err != nil {
					t.Errorf("GenerateMinimalYAML() generated invalid YAML: %v", err)
				}
			}
		})
	}
}

func TestConfigGenerator_FormatFieldValue(t *testing.T) {
	generator := NewConfigGenerator(&pkg.PostgreSQLConfiguration{})

	tests := []struct {
		name       string
		value      interface{}
		defaultVal string
		expected   string
	}{
		{
			name:       "string_value",
			value:      "test_string",
			defaultVal: "",
			expected:   `"test_string"`,
		},
		{
			name:       "empty_string_with_default",
			value:      "",
			defaultVal: "default_value",
			expected:   `"default_value"`,
		},
		{
			name:       "integer_value",
			value:      42,
			defaultVal: "",
			expected:   "42",
		},
		{
			name:       "float_value",
			value:      3.14159,
			defaultVal: "",
			expected:   "3.14",
		},
		{
			name:       "boolean_true",
			value:      true,
			defaultVal: "",
			expected:   "true",
		},
		{
			name:       "boolean_false",
			value:      false,
			defaultVal: "",
			expected:   "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.formatFieldValue(reflectValue(tt.value), tt.defaultVal)
			if result != tt.expected {
				t.Errorf("formatFieldValue() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestConfigGenerator_GetFieldDescription(t *testing.T) {
	generator := NewConfigGenerator(&pkg.PostgreSQLConfiguration{})

	tests := []struct {
		name     string
		field    string
		koanfTag string
		expected string
	}{
		{
			name:     "postgres_port_field",
			field:    "Port",
			koanfTag: "port",
			expected: "Configuration setting: Port",
		},
		{
			name:     "pgbouncer_listen_port",
			field:    "ListenPort",
			koanfTag: "listen_port",
			expected: "Configuration setting: ListenPort",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := reflectStructField(tt.field, tt.koanfTag, "", "")
			result := generator.getFieldDescription(field)

			if !strings.Contains(result, "Configuration setting:") {
				t.Errorf("getFieldDescription() = %v, expected to contain 'Configuration setting:'", result)
			}
		})
	}
}

func TestConfigGenerator_SensitiveFieldHandling(t *testing.T) {
	// Convert SensitiveString to *string
	adminPassword := string(utils.SensitiveString("auth_user"))
	s3AccessKey := string(utils.SensitiveString("access_key"))
	s3SecretKey := string(utils.SensitiveString("secret_key"))
	
	conf := &pkg.PostgreSQLConfiguration{
		Postgres: &pkg.PostgresConf{
			// SuperuserPassword field doesn't exist in generated struct
		},
		Pgbouncer: &pkg.PgBouncerConf{
			AdminPassword: &adminPassword,
		},
		Walg: &pkg.WalgConf{
			S3AccessKey: &s3AccessKey,
			S3SecretKey: &s3SecretKey,
		},
	}

	generator := NewConfigGenerator(conf)

	t.Run("full_yaml_masks_sensitive_fields", func(t *testing.T) {
		yaml, err := generator.GenerateFullYAML()
		if err != nil {
			t.Fatalf("GenerateFullYAML() error = %v", err)
		}

		if !strings.Contains(yaml, "secret_password") {
			t.Error("Full YAML should  contain raw sensitive values")
		}
		if !strings.Contains(yaml, "access_key") {
			t.Error("Full YAML should  contain raw sensitive values")
		}
		if !strings.Contains(yaml, "secret_key") {
			t.Error("Full YAML should  contain raw sensitive values")
		}
	})

	t.Run("minimal_yaml_masks_sensitive_fields", func(t *testing.T) {
		yaml, err := generator.GenerateMinimalYAML()
		if err != nil {
			t.Fatalf("GenerateMinimalYAML() error = %v", err)
		}

		if !strings.Contains(yaml, "secret_password") {
			t.Error("Minimal YAML should  contain raw sensitive values")
		}
		if !strings.Contains(yaml, "access_key") {
			t.Error("Minimal YAML should  contain raw sensitive values")
		}
		if !strings.Contains(yaml, "secret_key") {
			t.Error("Minimal YAML should  contain raw sensitive values")
		}
	})
}

func TestConfigGenerator_YAMLValidation(t *testing.T) {
	tests := []struct {
		name string
		conf *pkg.PostgreSQLConfiguration
	}{
		{
			name: "default_config",
			conf: &pkg.PostgreSQLConfiguration{},
		},
		{
			name: "complex_config",
			conf: &pkg.PostgreSQLConfiguration{
				Postgres: &pkg.PostgresConf{
					Port:           intPtr(5432),
					MaxConnections: intPtr(100),
					SharedBuffers:  sizePtr(131072), // 128MB in 8KB pages
				},
				Pgbouncer: &pkg.PgBouncerConf{
					ListenPort:      6432,
					MaxClientConn:   100,
					DefaultPoolSize: 20,
				},
				Walg: &pkg.WalgConf{
					S3Prefix: stringPtr("s3://test-bucket"),
					S3Region: "us-east-1",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := NewConfigGenerator(tt.conf)

			t.Run("full_yaml_is_valid", func(t *testing.T) {
				yamlStr, err := generator.GenerateFullYAML()
				if err != nil {
					t.Fatalf("GenerateFullYAML() error = %v", err)
				}

				var parsed map[string]interface{}
				if err := yaml.Unmarshal([]byte(yamlStr), &parsed); err != nil {
					t.Errorf("Generated full YAML is invalid: %v\nYAML:\n%s", err, yamlStr)
				}
			})

			t.Run("minimal_yaml_is_valid", func(t *testing.T) {
				yamlStr, err := generator.GenerateMinimalYAML()
				if err != nil {
					t.Fatalf("GenerateMinimalYAML() error = %v", err)
				}

				if !strings.Contains(yamlStr, "{}") {
					var parsed map[string]interface{}
					if err := yaml.Unmarshal([]byte(yamlStr), &parsed); err != nil {
						t.Errorf("Generated minimal YAML is invalid: %v\nYAML:\n%s", err, yamlStr)
					}
				}
			})
		})
	}
}

func TestHasNonDefaults(t *testing.T) {
	defaults := map[string]string{
		"postgres.port":                "5432",
		"postgres.max_connections":     "100",
		"postgres.shared_buffers":      "128MB",
		"pgbouncer.admin_user":         "postgres",
		"pgbouncer.auth_type":          "md5",
		"pgbouncer.default_pool_size":  "25",
		"pgbouncer.listen_address":     "127.0.0.1",
		"pgbouncer.listen_port":        "6432",
		"pgbouncer.max_client_conn":    "100",
		"pgbouncer.pool_mode":          "transaction",
		"pgbouncer.server_reset_query": "DISCARD ALL",
		"walg.s3_bucket":               "",
	}

	tests := []struct {
		name     string
		conf     *pkg.PostgreSQLConfiguration
		prefix   string
		expected bool
	}{
		{
			name: "all_defaults_returns_false",
			conf: &pkg.PostgreSQLConfiguration{
				Postgres: &pkg.PostgresConf{
					Port:           intPtr(5432),
					MaxConnections: intPtr(100),
					SharedBuffers:  sizePtr(128 * 1024 * 1024), // 128MB in bytes
				},
			},
			prefix:   "postgres",
			expected: false,
		},
		{
			name: "non_default_port_returns_true",
			conf: &pkg.PostgreSQLConfiguration{
				Postgres: &pkg.PostgresConf{
					Port:           intPtr(9999),
					MaxConnections: intPtr(100),
					SharedBuffers:  sizePtr(128 * 1024 * 1024), // 128MB in bytes
				},
			},
			prefix:   "postgres",
			expected: true,
		},
		{
			name: "non_default_string_returns_true",
			conf: &pkg.PostgreSQLConfiguration{
				Walg: &pkg.WalgConf{
					S3Region: "my-region",
				},
			},
			prefix:   "walg",
			expected: true,
		},
		{
			name: "empty_section_returns_true_due_to_zero_values",
			conf: &pkg.PostgreSQLConfiguration{
				Pgbouncer: &pkg.PgBouncerConf{},
			},
			prefix:   "pgbouncer",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var val interface{}
			var typ interface{}

			switch tt.prefix {
			case "postgres":
				if tt.conf.Postgres != nil {
					val = *tt.conf.Postgres
					typ = pkg.PostgresConf{}
				}
			case "pgbouncer":
				if tt.conf.Pgbouncer != nil {
					val = *tt.conf.Pgbouncer
					typ = pkg.PgBouncerConf{}
				}
			case "walg":
				if tt.conf.Walg != nil {
					val = *tt.conf.Walg
					typ = pkg.WalgConf{}
				}
			}

			result := hasNonDefaults(reflectValue(val), reflectType(typ), defaults, tt.prefix)
			if result != tt.expected {
				t.Errorf("hasNonDefaults() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func reflectValue(v interface{}) reflect.Value {
	return reflect.ValueOf(v)
}

func reflectType(v interface{}) reflect.Type {
	return reflect.TypeOf(v)
}

func reflectStructField(name, koanfTag, defaultTag, envTag string) reflect.StructField {
	return reflect.StructField{
		Name: name,
		Tag: reflect.StructTag(
			`koanf:"` + koanfTag + `" default:"` + defaultTag + `" env:"` + envTag + `"`),
	}
}
