package pkg

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestConfigurationIntegration(t *testing.T) {
	// Create a temporary directory for test outputs
	tempDir, err := os.MkdirTemp("", "pgconfig-integration-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("configuration_loading_integration", func(t *testing.T) {
		// Test loading a complete configuration
		testConfig := map[string]interface{}{
			"postgres": map[string]interface{}{
				"port":            5432,
				"max_connections": 100,
				"shared_buffers":  "128MB",
			},
			"pgbouncer": map[string]interface{}{
				"listen_port":       6432,
				"max_client_conn":   100,
				"default_pool_size": 20,
			},
			"walg": map[string]interface{}{
				"s3_prefix": "s3://my-backups/path",
				"s3_region": "us-east-1",
			},
		}

		// Create test YAML file
		yamlData, err := yaml.Marshal(testConfig)
		if err != nil {
			t.Fatalf("Failed to marshal test config: %v", err)
		}

		configFile := filepath.Join(tempDir, "test-config.yaml")
		if err := os.WriteFile(configFile, yamlData, 0644); err != nil {
			t.Fatalf("Failed to write test config file: %v", err)
		}

		// Test loading configuration with schema validation
		conf, err := LoadConfigWithValidation(configFile, "../schema/pgconfig-schema.json")
		if err != nil {
			t.Errorf("Failed to load valid configuration: %v", err)
			return
		}

		// Verify loaded values
		if conf.Postgres == nil {
			t.Error("Postgres configuration should be loaded")
		} else {
			if conf.Postgres.Port != 5432 {
				t.Errorf("Expected port 5432, got %v", conf.Postgres.Port)
			}
			if conf.Postgres.MaxConnections != 100 {
				t.Errorf("Expected max_connections 100, got %v", conf.Postgres.MaxConnections)
			}
		}

		if conf.Pgbouncer == nil {
			t.Error("PgBouncer configuration should be loaded")
		} else {
			if conf.Pgbouncer.ListenPort != 6432 {
				t.Errorf("Expected listen_port 6432, got %d", conf.Pgbouncer.ListenPort)
			}
		}

		if conf.Walg == nil {
			t.Error("WAL-G configuration should be loaded")
		} else {
			if conf.Walg.S3Prefix == nil || *conf.Walg.S3Prefix != "s3://my-backups/path" {
				t.Errorf("Expected S3 prefix 's3://my-backups/path', got %v", conf.Walg.S3Prefix)
			}
		}
	})

	t.Run("invalid_configuration_rejection", func(t *testing.T) {
		// Test that invalid configurations are properly rejected
		invalidConfigs := []struct {
			name        string
			config      map[string]interface{}
			expectError string
		}{
			{
				name: "invalid_postgres_field",
				config: map[string]interface{}{
					"postgres": map[string]interface{}{
						"invalid_field_name": "some_value",
					},
				},
				expectError: "Additional property invalid_field_name is not allowed",
			},
			{
				name: "invalid_port_type",
				config: map[string]interface{}{
					"postgres": map[string]interface{}{
						"port": "not_a_number",
					},
				},
				expectError: "Invalid type. Expected: integer, given: string",
			},
			{
				name: "port_out_of_range",
				config: map[string]interface{}{
					"postgres": map[string]interface{}{
						"port": 99999,
					},
				},
				expectError: "Must be less than or equal to",
			},
		}

		for _, tc := range invalidConfigs {
			t.Run(tc.name, func(t *testing.T) {
				yamlData, err := yaml.Marshal(tc.config)
				if err != nil {
					t.Fatalf("Failed to marshal test config: %v", err)
				}

				configFile := filepath.Join(tempDir, "invalid-config.yaml")
				if err := os.WriteFile(configFile, yamlData, 0644); err != nil {
					t.Fatalf("Failed to write test config file: %v", err)
				}

				// This should fail with validation error
				_, err = LoadConfigWithValidation(configFile, "../schema/pgconfig-schema.json")
				if err == nil {
					t.Errorf("Expected validation error for %s, but got none", tc.name)
					return
				}

				if !strings.Contains(err.Error(), tc.expectError) {
					t.Errorf("Expected error containing '%s', got: %v", tc.expectError, err)
				}
			})
		}
	})

	t.Run("environment_variable_resolution", func(t *testing.T) {
		// Test that environment variables are properly resolved in defaults
		testConfig := map[string]interface{}{
			"postgres": map[string]interface{}{
				"port": 5432,
			},
		}

		yamlData, err := yaml.Marshal(testConfig)
		if err != nil {
			t.Fatalf("Failed to marshal test config: %v", err)
		}

		configFile := filepath.Join(tempDir, "env-test-config.yaml")
		if err := os.WriteFile(configFile, yamlData, 0644); err != nil {
			t.Fatalf("Failed to write test config file: %v", err)
		}

		// Set environment variable for testing
		os.Setenv("POSTGRES_MAX_CONNECTIONS", "150")
		defer os.Unsetenv("POSTGRES_MAX_CONNECTIONS")

		conf, err := LoadConfigWithValidation(configFile, "../schema/pgconfig-schema.json")
		if err != nil {
			t.Errorf("Failed to load configuration: %v", err)
			return
		}

		// Environment variable should override the default
		if conf.Postgres.MaxConnections != 150 {
			t.Errorf("Expected max_connections from env var (150), got %v", conf.Postgres.MaxConnections)
		}
	})

	t.Run("defaults_application", func(t *testing.T) {
		// Test that defaults are properly applied when fields are missing
		minimalConfig := map[string]interface{}{
			"postgres": map[string]interface{}{
				"port": 5433, // Only specify port
			},
		}

		yamlData, err := yaml.Marshal(minimalConfig)
		if err != nil {
			t.Fatalf("Failed to marshal test config: %v", err)
		}

		configFile := filepath.Join(tempDir, "minimal-config.yaml")
		if err := os.WriteFile(configFile, yamlData, 0644); err != nil {
			t.Fatalf("Failed to write test config file: %v", err)
		}

		conf, err := LoadConfigWithValidation(configFile, "../schema/pgconfig-schema.json")
		if err != nil {
			t.Errorf("Failed to load minimal configuration: %v", err)
			return
		}

		// Verify defaults are applied
		if conf.Postgres.Port != 5433 {
			t.Errorf("Expected custom port 5433, got %v", conf.Postgres.Port)
		}

		// Check that defaults are applied for unspecified fields
		if conf.Postgres.MaxConnections == 0 {
			t.Error("Expected default max_connections to be applied, got 0")
		}

		if conf.Postgres.SharedBuffers == "" {
			t.Error("Expected default shared_buffers to be applied, got empty string")
		}
	})
}

func TestPgconfigCLIIntegration(t *testing.T) {
	// Test the pgconfig CLI tool functionality
	tempDir, err := os.MkdirTemp("", "pgconfig-cli-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Build the pgconfig CLI tool first
	buildCmd := exec.Command("task", "build-pgconfig")
	buildCmd.Dir = ".."
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Skipf("Could not build pgconfig CLI: %v\nOutput: %s", err, string(output))
	}

	// Check if pgconfig binary exists
	pgconfigPath := "../pgconfig"
	if _, err := os.Stat(pgconfigPath); os.IsNotExist(err) {
		t.Skipf("pgconfig CLI not found at %s", pgconfigPath)
	}

	t.Run("generate_command", func(t *testing.T) {
		// Test generating configuration files
		cmd := exec.Command(pgconfigPath, "generate", "--output-dir", tempDir)
		cmd.Dir = ".."
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("pgconfig generate failed: %v\nOutput: %s", err, string(output))
			return
		}

		// Check that expected configuration files are generated
		expectedFiles := []string{
			"postgresql.conf",
			"pgbouncer.ini",
			"postgrest.conf",
		}

		for _, filename := range expectedFiles {
			filePath := filepath.Join(tempDir, filename)
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				t.Errorf("Expected generated file not found: %s", filename)
			} else {
				// Verify file has content
				content, err := os.ReadFile(filePath)
				if err != nil {
					t.Errorf("Failed to read generated file %s: %v", filename, err)
				} else if len(content) == 0 {
					t.Errorf("Generated file %s is empty", filename)
				}
			}
		}
	})

	t.Run("validate_command", func(t *testing.T) {
		// Create a test configuration file
		testConfig := map[string]interface{}{
			"postgres": map[string]interface{}{
				"port":            5432,
				"max_connections": 100,
			},
		}

		yamlData, err := yaml.Marshal(testConfig)
		if err != nil {
			t.Fatalf("Failed to marshal test config: %v", err)
		}

		configFile := filepath.Join(tempDir, "test-config.yaml")
		if err := os.WriteFile(configFile, yamlData, 0644); err != nil {
			t.Fatalf("Failed to write test config file: %v", err)
		}

		// Test validating the configuration
		cmd := exec.Command(pgconfigPath, "validate", "--file", configFile)
		cmd.Dir = ".."
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("pgconfig validate failed on valid config: %v\nOutput: %s", err, string(output))
		}

		// Test validating an invalid configuration
		invalidConfig := map[string]interface{}{
			"postgres": map[string]interface{}{
				"invalid_field": "value",
			},
		}

		invalidYamlData, err := yaml.Marshal(invalidConfig)
		if err != nil {
			t.Fatalf("Failed to marshal invalid config: %v", err)
		}

		invalidConfigFile := filepath.Join(tempDir, "invalid-config.yaml")
		if err := os.WriteFile(invalidConfigFile, invalidYamlData, 0644); err != nil {
			t.Fatalf("Failed to write invalid config file: %v", err)
		}

		cmd = exec.Command(pgconfigPath, "validate", "--file", invalidConfigFile)
		cmd.Dir = ".."
		output, err = cmd.CombinedOutput()
		if err == nil {
			t.Error("pgconfig validate should have failed on invalid config")
		}
	})
}
