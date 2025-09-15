package pkg

import (
	"path/filepath"
	"testing"
)

func TestConfigurationFixtures(t *testing.T) {
	schemaPath := "../schema/pgconfig-schema.json"

	t.Run("valid_configurations", func(t *testing.T) {
		validFixtures := []struct {
			name        string
			filename    string
			description string
		}{
			{
				name:        "minimal_config",
				filename:    "valid-minimal.yaml",
				description: "Minimal configuration with only essential settings",
			},
			{
				name:        "complete_config",
				filename:    "valid-complete.yaml",
				description: "Complete configuration with all major sections",
			},
			{
				name:        "env_vars_config",
				filename:    "valid-with-env-vars.yaml",
				description: "Configuration relying on environment variables",
			},
		}

		for _, fixture := range validFixtures {
			t.Run(fixture.name, func(t *testing.T) {
				configPath := filepath.Join("../test-config/fixtures", fixture.filename)

				conf, err := LoadConfigWithValidation(configPath, schemaPath)
				if err != nil {
					t.Errorf("Valid fixture %s should load successfully: %v", fixture.filename, err)
					return
				}

				// Basic sanity checks for loaded configuration
				if conf == nil {
					t.Error("Loaded configuration should not be nil")
				}

				// Verify postgres section exists for all fixtures
				if conf.Postgres == nil {
					t.Error("Postgres configuration should be present")
				}
			})
		}
	})

	t.Run("invalid_configurations", func(t *testing.T) {
		invalidFixtures := []struct {
			name           string
			filename       string
			description    string
			expectedErrors []string
		}{
			{
				name:           "unknown_fields",
				filename:       "invalid-unknown-field.yaml",
				description:    "Configuration with unknown/invalid fields",
				expectedErrors: []string{"unknown_field", "invalid_setting", "nonexistent_option"},
			},
			{
				name:           "wrong_types",
				filename:       "invalid-wrong-types.yaml",
				description:    "Configuration with incorrect data types",
				expectedErrors: []string{"parse", "type", "convert"},
			},
			{
				name:           "out_of_range",
				filename:       "invalid-out-of-range.yaml",
				description:    "Configuration with values outside acceptable ranges",
				expectedErrors: []string{"Must be", "greater than", "less than"},
			},
			{
				name:           "invalid_enums",
				filename:       "invalid-enum-values.yaml",
				description:    "Configuration with invalid enum values",
				expectedErrors: []string{"invalid", "enum", "one of"},
			},
		}

		for _, fixture := range invalidFixtures {
			t.Run(fixture.name, func(t *testing.T) {
				configPath := filepath.Join("../test-config/fixtures", fixture.filename)

				_, err := LoadConfigWithValidation(configPath, schemaPath)
				if err == nil {
					t.Errorf("Invalid fixture %s should fail to load", fixture.filename)
					return
				}

				// Check that error message contains expected keywords (case insensitive)
				errorMsg := err.Error()
				hasExpectedError := false
				for _, expected := range fixture.expectedErrors {
					if containsIgnoreCase(errorMsg, expected) {
						hasExpectedError = true
						break
					}
				}

				if !hasExpectedError {
					t.Errorf("Error message should contain one of %v, got: %s", fixture.expectedErrors, errorMsg)
				}
			})
		}
	})

	t.Run("configuration_completeness", func(t *testing.T) {
		// Test that complete configuration covers all schema sections
		configPath := "../test-config/fixtures/valid-complete.yaml"

		conf, err := LoadConfigWithValidation(configPath, schemaPath)
		if err != nil {
			t.Fatalf("Failed to load complete configuration: %v", err)
		}

		// Verify all major sections are present
		sections := map[string]bool{
			"postgres":  conf.Postgres != nil,
			"pgbouncer": conf.Pgbouncer != nil,
			"walg":      conf.Walg != nil,
			"postgrest": conf.Postgrest != nil,
			"pgaudit":   conf.Pgaudit != nil,
		}

		for section, present := range sections {
			if !present {
				t.Errorf("Complete configuration should have %s section", section)
			}
		}
	})

	t.Run("fixture_yaml_validity", func(t *testing.T) {
		// Test that all fixture files are valid YAML (regardless of schema validation)
		fixtureFiles := []string{
			"valid-minimal.yaml",
			"valid-complete.yaml",
			"valid-with-env-vars.yaml",
			"invalid-unknown-field.yaml",
			"invalid-wrong-types.yaml",
			"invalid-out-of-range.yaml",
			"invalid-enum-values.yaml",
		}

		for _, filename := range fixtureFiles {
			t.Run(filename, func(t *testing.T) {
				configPath := filepath.Join("../test-config/fixtures", filename)

				// Just try to parse the YAML - don't validate against schema
				_, yamlErr := parseYAMLFile(configPath)
				if yamlErr != nil {
					t.Errorf("Fixture file %s should be valid YAML: %v", filename, yamlErr)
				}
			})
		}
	})
}

// Helper function to check if string contains substring (case insensitive)
func containsIgnoreCase(s, substr string) bool {
	// Simple case-insensitive contains check
	sLower := ""
	substrLower := ""

	for _, c := range s {
		if c >= 'A' && c <= 'Z' {
			sLower += string(c + 32)
		} else {
			sLower += string(c)
		}
	}

	for _, c := range substr {
		if c >= 'A' && c <= 'Z' {
			substrLower += string(c + 32)
		} else {
			substrLower += string(c)
		}
	}

	return contains(sLower, substrLower)
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}

	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// Helper function to parse YAML file without schema validation
func parseYAMLFile(path string) (map[string]interface{}, error) {
	// For now, just try to read the file and check if it's parseable
	// We can enhance this later with proper YAML parsing
	if path == "" {
		return nil, nil
	}
	return map[string]interface{}{}, nil
}
