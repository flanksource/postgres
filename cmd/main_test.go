// Tests how the CLI picks its default connection settings from the environment.
// Covers the username fallback shared with docker-entrypoint.sh's initdb.
package main

import "testing"

func TestDefaultUsername(t *testing.T) {
	tests := []struct {
		name         string
		pgUser       string
		postgresUser string
		expected     string
	}{
		{"PG_USER wins", "admin", "xero", "admin"},
		{"falls back to POSTGRES_USER used by initdb", "", "xero", "xero"},
		{"defaults to postgres", "", "", "postgres"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PG_USER", tt.pgUser)
			t.Setenv("POSTGRES_USER", tt.postgresUser)
			if got := defaultUsername(); got != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, got)
			}
		})
	}
}
