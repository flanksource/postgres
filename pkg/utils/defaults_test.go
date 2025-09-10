package utils

import (
	"os"
	"testing"
)

func TestResolveDefault(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		envVar   string
		envValue string
		expected string
	}{
		{
			name:     "plain text",
			input:    "5432",
			expected: "5432",
		},
		{
			name:     "env var with default",
			input:    "${POSTGRES_PORT:-5432}",
			expected: "5432",
		},
		{
			name:     "env var with default and set value",
			input:    "${POSTGRES_PORT:-5432}",
			envVar:   "POSTGRES_PORT",
			envValue: "9999",
			expected: "9999",
		},
		{
			name:     "env var without default",
			input:    "${POSTGRES_HOST}",
			expected: "",
		},
		{
			name:     "env var without default and set value",
			input:    "${POSTGRES_HOST}",
			envVar:   "POSTGRES_HOST",
			envValue: "myhost",
			expected: "myhost",
		},
		{
			name:     "complex default with special chars",
			input:    "${POSTGRES_SSL_CIPHERS:-HIGH:MEDIUM:+3DES:!aNULL}",
			expected: "HIGH:MEDIUM:+3DES:!aNULL",
		},
		{
			name:     "multiple env vars",
			input:    "${HOST:-localhost}:${PORT:-5432}",
			expected: "localhost:5432",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variable if specified
			if tt.envVar != "" {
				oldValue := os.Getenv(tt.envVar)
				os.Setenv(tt.envVar, tt.envValue)
				defer func() {
					if oldValue == "" {
						os.Unsetenv(tt.envVar)
					} else {
						os.Setenv(tt.envVar, oldValue)
					}
				}()
			}

			result := ResolveDefault(tt.input)
			if result != tt.expected {
				t.Errorf("ResolveDefault(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestResolveBoolDefault(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		fallback bool
		envVar   string
		envValue string
		expected bool
	}{
		{
			name:     "default fallback",
			input:    "",
			fallback: true,
			expected: true,
		},
		{
			name:     "env var true",
			input:    "${POSTGRES_SSL:-false}",
			envVar:   "POSTGRES_SSL",
			envValue: "true",
			expected: true,
		},
		{
			name:     "env var false",
			input:    "${POSTGRES_SSL:-true}",
			envVar:   "POSTGRES_SSL",
			envValue: "false",
			expected: false,
		},
		{
			name:     "schema default",
			input:    "${POSTGRES_SSL:-true}",
			fallback: false,
			expected: true,
		},
		{
			name:     "invalid value uses fallback",
			input:    "${POSTGRES_SSL:-invalid}",
			fallback: true,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variable if specified
			if tt.envVar != "" {
				oldValue := os.Getenv(tt.envVar)
				os.Setenv(tt.envVar, tt.envValue)
				defer func() {
					if oldValue == "" {
						os.Unsetenv(tt.envVar)
					} else {
						os.Setenv(tt.envVar, oldValue)
					}
				}()
			}

			result := ResolveBoolDefault(tt.input, tt.fallback)
			if result != tt.expected {
				t.Errorf("ResolveBoolDefault(%q, %v) = %v, expected %v", tt.input, tt.fallback, result, tt.expected)
			}
		})
	}
}

func TestResolveIntDefault(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		fallback int
		envVar   string
		envValue string
		expected int
	}{
		{
			name:     "default fallback",
			input:    "",
			fallback: 100,
			expected: 100,
		},
		{
			name:     "env var number",
			input:    "${POSTGRES_PORT:-5432}",
			envVar:   "POSTGRES_PORT",
			envValue: "9999",
			expected: 9999,
		},
		{
			name:     "schema default",
			input:    "${POSTGRES_PORT:-5432}",
			fallback: 80,
			expected: 5432,
		},
		{
			name:     "invalid value uses fallback",
			input:    "${POSTGRES_PORT:-invalid}",
			fallback: 8080,
			expected: 8080,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variable if specified
			if tt.envVar != "" {
				oldValue := os.Getenv(tt.envVar)
				os.Setenv(tt.envVar, tt.envValue)
				defer func() {
					if oldValue == "" {
						os.Unsetenv(tt.envVar)
					} else {
						os.Setenv(tt.envVar, oldValue)
					}
				}()
			}

			result := ResolveIntDefault(tt.input, tt.fallback)
			if result != tt.expected {
				t.Errorf("ResolveIntDefault(%q, %v) = %v, expected %v", tt.input, tt.fallback, result, tt.expected)
			}
		})
	}
}

func TestResolveFloat64Default(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		fallback float64
		envVar   string
		envValue string
		expected float64
	}{
		{
			name:     "default fallback",
			input:    "",
			fallback: 4.0,
			expected: 4.0,
		},
		{
			name:     "env var float",
			input:    "${POSTGRES_RANDOM_PAGE_COST:-4.0}",
			envVar:   "POSTGRES_RANDOM_PAGE_COST",
			envValue: "1.5",
			expected: 1.5,
		},
		{
			name:     "schema default",
			input:    "${POSTGRES_RANDOM_PAGE_COST:-0.9}",
			fallback: 1.0,
			expected: 0.9,
		},
		{
			name:     "invalid value uses fallback",
			input:    "${POSTGRES_RANDOM_PAGE_COST:-invalid}",
			fallback: 2.5,
			expected: 2.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variable if specified
			if tt.envVar != "" {
				oldValue := os.Getenv(tt.envVar)
				os.Setenv(tt.envVar, tt.envValue)
				defer func() {
					if oldValue == "" {
						os.Unsetenv(tt.envVar)
					} else {
						os.Setenv(tt.envVar, oldValue)
					}
				}()
			}

			result := ResolveFloat64Default(tt.input, tt.fallback)
			if result != tt.expected {
				t.Errorf("ResolveFloat64Default(%q, %v) = %v, expected %v", tt.input, tt.fallback, result, tt.expected)
			}
		})
	}
}

func TestGetSchemaDefaults(t *testing.T) {
	defaults := GetSchemaDefaults()
	
	// Test a few key defaults to ensure they're present
	expectedKeys := []string{
		"postgres.port",
		"postgres.max_connections", 
		"postgres.shared_buffers",
		"pgbouncer.listen_port",
		"postgrest.server_port",
		"walg.enabled",
		"pgaudit.log",
	}
	
	for _, key := range expectedKeys {
		if _, exists := defaults[key]; !exists {
			t.Errorf("GetSchemaDefaults() missing expected key: %s", key)
		}
	}
	
	// Test that postgres.port has the expected format
	if portDefault, exists := defaults["postgres.port"]; exists {
		expected := "${POSTGRES_PORT:-5432}"
		if portDefault != expected {
			t.Errorf("postgres.port default = %q, expected %q", portDefault, expected)
		}
	}
}