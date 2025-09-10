package pkg

import (
	"testing"
)

func TestPostgresValidate(t *testing.T) {
	postgres := NewPostgres(nil, "")

	tests := []struct {
		name      string
		config    string
		shouldErr bool
		errMsg    string
	}{
		{
			name: "valid basic config",
			config: `# Basic PostgreSQL configuration
max_connections = 100
shared_buffers = 128MB
effective_cache_size = 512MB`,
			shouldErr: false,
		},
		{
			name: "invalid parameter name",
			config: `# Invalid parameter
max_connections = 100
invalid_parameter = 123`,
			shouldErr: true,
			errMsg:    "unrecognized configuration parameter",
		},
		{
			name: "invalid parameter value",
			config: `# Invalid value
max_connections = invalid_value`,
			shouldErr: true,
			errMsg:    "invalid value",
		},
		{
			name: "parameter out of range",
			config: `# Out of range
max_connections = -1`,
			shouldErr: true,
			errMsg:    "outside the valid range",
		},
		{
			name: "empty config",
			config: `# Empty config with just comments
# This should be valid`,
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := postgres.Validate([]byte(tt.config))

			if tt.shouldErr {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}

				if tt.errMsg != "" {
					if !containsString(err.Error(), tt.errMsg) {
						t.Errorf("Expected error message to contain '%s', got: %s", tt.errMsg, err.Error())
					}
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %s", err.Error())
				}
			}
		})
	}
}

func TestPgBouncerValidate(t *testing.T) {
	pgbouncer := NewPgBouncer(nil)

	tests := []struct {
		name      string
		config    string
		shouldErr bool
		errMsg    string
	}{
		{
			name: "valid basic config",
			config: `[databases]
testdb = host=localhost port=5432 dbname=testdb

[pgbouncer]
listen_addr = 0.0.0.0
listen_port = 6432
pool_mode = transaction`,
			shouldErr: false,
		},
		{
			name: "missing databases section",
			config: `[pgbouncer]
listen_addr = 0.0.0.0
listen_port = 6432`,
			shouldErr: true,
			errMsg:    "missing required [databases] section",
		},
		{
			name: "missing pgbouncer section",
			config: `[databases]
testdb = host=localhost port=5432 dbname=testdb`,
			shouldErr: true,
			errMsg:    "missing required [pgbouncer] section",
		},
		{
			name: "invalid connection string",
			config: `[databases]
testdb = invalid_connection_string

[pgbouncer]
listen_addr = 0.0.0.0`,
			shouldErr: true,
			errMsg:    "missing",
		},
		{
			name: "invalid line format",
			config: `[databases]
invalid line without equals

[pgbouncer]
listen_addr = 0.0.0.0`,
			shouldErr: true,
			errMsg:    "invalid line format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pgbouncer.Validate([]byte(tt.config))

			if tt.shouldErr {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}

				if tt.errMsg != "" {
					if !containsString(err.Error(), tt.errMsg) {
						t.Errorf("Expected error message to contain '%s', got: %s", tt.errMsg, err.Error())
					}
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %s", err.Error())
				}
			}
		})
	}
}

func TestPostgRESTValidate(t *testing.T) {
	postgrest := NewPostgREST(nil)

	tests := []struct {
		name      string
		config    string
		shouldErr bool
		errMsg    string
	}{
		{
			name: "valid basic config",
			config: `db-uri = postgres://user:pass@localhost:5432/dbname
db-schema = public
db-anon-role = anon_role
server-host = localhost
server-port = 3000`,
			shouldErr: false,
		},
		{
			name: "missing required parameter",
			config: `db-schema = public
db-anon-role = anon_role`,
			shouldErr: true,
			errMsg:    "required parameter missing",
		},
		{
			name: "invalid db-uri format",
			config: `db-uri = invalid_uri_format
db-schema = public
db-anon-role = anon_role`,
			shouldErr: true,
			errMsg:    "must start with postgres://",
		},
		{
			name: "invalid schema name",
			config: `db-uri = postgres://user:pass@localhost:5432/dbname
db-schema = 123invalid
db-anon-role = anon_role`,
			shouldErr: true,
			errMsg:    "invalid schema name format",
		},
		{
			name: "invalid port",
			config: `db-uri = postgres://user:pass@localhost:5432/dbname
db-schema = public
db-anon-role = anon_role
server-port = invalid_port`,
			shouldErr: true,
			errMsg:    "port must be a number",
		},
		{
			name: "empty required parameter",
			config: `db-uri = 
db-schema = public
db-anon-role = anon_role`,
			shouldErr: true,
			errMsg:    "database URI cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := postgrest.Validate([]byte(tt.config))

			if tt.shouldErr {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}

				if tt.errMsg != "" {
					if !containsString(err.Error(), tt.errMsg) {
						t.Errorf("Expected error message to contain '%s', got: %s", tt.errMsg, err.Error())
					}
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %s", err.Error())
				}
			}
		})
	}
}

func TestValidationError(t *testing.T) {
	tests := []struct {
		name     string
		err      *ValidationError
		expected string
	}{
		{
			name: "error with line and parameter",
			err: &ValidationError{
				Line:      10,
				Parameter: "max_connections",
				Message:   "invalid value",
				Raw:       "original error",
			},
			expected: "line 10: max_connections: invalid value",
		},
		{
			name: "error with parameter only",
			err: &ValidationError{
				Parameter: "db-uri",
				Message:   "required parameter missing",
			},
			expected: "db-uri: required parameter missing",
		},
		{
			name: "error with message only",
			err: &ValidationError{
				Message: "configuration contains errors",
			},
			expected: "configuration contains errors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.err.Error()
			if actual != tt.expected {
				t.Errorf("Expected error message '%s', got '%s'", tt.expected, actual)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				indexOfString(s, substr) >= 0)))
}

// Helper function to find substring index
func indexOfString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
