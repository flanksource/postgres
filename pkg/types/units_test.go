package types

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSizeParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected uint64
	}{
		// Standard PostgreSQL size formats
		{"128MB", 128 * 1024 * 1024},
		{"1GB", 1024 * 1024 * 1024},
		{"512kB", 512 * 1024},
		{"4TB", 4 * 1024 * 1024 * 1024 * 1024},

		// PostgreSQL defaults from config.txt
		{"64MB", 64 * 1024 * 1024},      // maintenance_work_mem default
		{"4MB", 4 * 1024 * 1024},        // work_mem default
		{"16MB", 16 * 1024 * 1024},      // wal_buffers default
		{"4GB", 4 * 1024 * 1024 * 1024}, // effective_cache_size default

		// Edge cases
		{"0", 0},
		{"1024", 1024}, // plain number assumed to be bytes
		{"1kB", 1024},
		{"1MB", 1024 * 1024},
		{"100GB", 100 * 1024 * 1024 * 1024},

		// PostgreSQL block sizes (8kB units)
		{"1024", 1024},   // shared_buffers in pages -> bytes
		{"16384", 16384}, // temp_buffers in pages -> bytes
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			size, err := ParseSize(test.input)
			if err != nil {
				t.Fatalf("Failed to parse %s: %v", test.input, err)
			}
			if size.Bytes() != test.expected {
				t.Errorf("Expected %d bytes, got %d", test.expected, size.Bytes())
			}
		})
	}
}

func TestSizeJSONMarshaling(t *testing.T) {
	size := Size(128 * 1024 * 1024) // 128MB

	// Marshal to JSON
	jsonData, err := json.Marshal(size)
	if err != nil {
		t.Fatalf("Failed to marshal size: %v", err)
	}

	// Should be a string representation
	expected := `"128MB"`
	if string(jsonData) != expected {
		t.Errorf("Expected JSON %s, got %s", expected, string(jsonData))
	}

	// Unmarshal back
	var unmarshaled Size
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal size: %v", err)
	}

	if unmarshaled != size {
		t.Errorf("Expected unmarshaled size %d, got %d", size, unmarshaled)
	}
}

func TestSizePostgreSQLString(t *testing.T) {
	tests := []struct {
		bytes    uint64
		expected string
	}{
		{128 * 1024 * 1024, "128MB"},
		{1024 * 1024 * 1024, "1024MB"},
		{512 * 1024, "1MB"}, // 512kB rounds up to 1MB
		{0, "0MB"},
	}

	for _, test := range tests {
		size := Size(test.bytes)
		result := size.PostgreSQLMB()
		if result != test.expected {
			t.Errorf("Expected PostgreSQL string %s, got %s", test.expected, result)
		}
	}
}

func TestDurationParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		// Standard PostgreSQL duration formats
		{"5min", 5 * time.Minute},
		{"30s", 30 * time.Second},
		{"1h", 1 * time.Hour},
		{"1d", 24 * time.Hour},

		// PostgreSQL defaults from config.txt
		{"60s", 60 * time.Second},         // authentication_timeout default
		{"30s", 30 * time.Second},         // checkpoint_timeout minimum
		{"5min", 5 * time.Minute},         // checkpoint_timeout typical
		{"1s", 1 * time.Second},           // deadlock_timeout minimum
		{"200ms", 200 * time.Millisecond}, // bgwriter_delay typical

		// Microseconds (PostgreSQL supports)
		{"500us", 500 * time.Microsecond},
		{"1ms", 1 * time.Millisecond},

		// Edge cases
		{"0", 0},
		{"1000", 1000 * time.Millisecond}, // plain number assumed to be milliseconds
		{"86400s", 86400 * time.Second},   // 1 day in seconds
		{"1440min", 1440 * time.Minute},   // 1 day in minutes
		{"24h", 24 * time.Hour},           // 1 day in hours

		// PostgreSQL vacuum and autovacuum delays
		{"20ms", 20 * time.Millisecond}, // vacuum_cost_delay default
		{"0ms", 0},                      // disabled delay
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			duration, err := ParseDuration(test.input)
			if err != nil {
				t.Fatalf("Failed to parse %s: %v", test.input, err)
			}
			if duration.Duration() != test.expected {
				t.Errorf("Expected %v, got %v", test.expected, duration.Duration())
			}
		})
	}
}

func TestDurationJSONMarshaling(t *testing.T) {
	duration := Duration(5 * time.Minute)

	// Marshal to JSON
	jsonData, err := json.Marshal(duration)
	if err != nil {
		t.Fatalf("Failed to marshal duration: %v", err)
	}

	// Should be a string representation
	expected := `"5.0min"`
	if string(jsonData) != expected {
		t.Errorf("Expected JSON %s, got %s", expected, string(jsonData))
	}

	// Unmarshal back
	var unmarshaled Duration
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal duration: %v", err)
	}

	if unmarshaled != duration {
		t.Errorf("Expected unmarshaled duration %v, got %v", duration, unmarshaled)
	}
}

func TestDurationPostgreSQLString(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{5 * time.Minute, "5min"},
		{30 * time.Second, "30s"},
		{1 * time.Hour, "1h"},
		{0, "0"},
	}

	for _, test := range tests {
		duration := Duration(test.duration)
		result := duration.PostgreSQLString()
		if result != test.expected {
			t.Errorf("Expected PostgreSQL string %s, got %s", test.expected, result)
		}
	}
}

func TestNewSize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected uint64
	}{
		{"128MB default", "128MB", 128 * 1024 * 1024},
		{"4GB effective_cache_size", "4GB", 4 * 1024 * 1024 * 1024},
		{"64MB maintenance_work_mem", "64MB", 64 * 1024 * 1024},
		{"4MB work_mem", "4MB", 4 * 1024 * 1024},
		{"16MB wal_buffers", "16MB", 16 * 1024 * 1024},
		{"zero size", "0", 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			size := NewSize(test.input)
			if size.Bytes() != test.expected {
				t.Errorf("Expected %d bytes, got %d", test.expected, size.Bytes())
			}
		})
	}
}

func TestNewSizePanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected NewSize to panic on invalid input")
		}
	}()
	NewSize("invalid size")
}

func TestNewDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
	}{
		{"60s authentication_timeout", "60s", 60 * time.Second},
		{"5min checkpoint_timeout", "5min", 5 * time.Minute},
		{"1s deadlock_timeout", "1s", 1 * time.Second},
		{"200ms bgwriter_delay", "200ms", 200 * time.Millisecond},
		{"30s idle timeout", "30s", 30 * time.Second},
		{"zero duration", "0", 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			duration := NewDuration(test.input)
			if duration.Duration() != test.expected {
				t.Errorf("Expected %v, got %v", test.expected, duration.Duration())
			}
		})
	}
}

func TestNewDurationPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected NewDuration to panic on invalid input")
		}
	}()
	NewDuration("invalid duration")
}

func TestPostgreSQLRealisticValues(t *testing.T) {
	// Test with typical PostgreSQL configuration values
	configs := []struct {
		name     string
		sizeStr  string
		expected string // PostgreSQL representation
	}{
		{"shared_buffers typical", "128MB", "128MB"},
		{"work_mem typical", "4MB", "4MB"},
		{"maintenance_work_mem typical", "64MB", "64MB"},
		{"effective_cache_size typical", "4GB", "4096MB"}, // Converted to MB
		{"wal_buffers typical", "16MB", "16MB"},
		{"temp_buffers typical", "8MB", "8MB"},
	}

	for _, config := range configs {
		t.Run(config.name, func(t *testing.T) {
			size := NewSize(config.sizeStr)
			pgString := size.PostgreSQLMB()
			if pgString != config.expected {
				t.Errorf("Expected PostgreSQL string %s, got %s", config.expected, pgString)
			}
		})
	}
}

func TestDurationRealisticPostgreSQLValues(t *testing.T) {
	// Test with typical PostgreSQL timeout/delay values
	configs := []struct {
		name        string
		durationStr string
		expected    string // PostgreSQL representation
	}{
		{"statement_timeout disabled", "0", "0"},
		{"authentication_timeout", "60s", "1min"}, // Duration formatting converts 60s to 1min
		{"checkpoint_timeout", "5min", "5min"},
		{"deadlock_timeout", "1s", "1s"},
		{"lock_timeout disabled", "0", "0"},
		{"idle_in_transaction_session_timeout", "30min", "30min"},
		{"bgwriter_delay", "200ms", "200ms"},
		{"vacuum_cost_delay", "20ms", "20ms"},
	}

	for _, config := range configs {
		t.Run(config.name, func(t *testing.T) {
			duration := NewDuration(config.durationStr)
			pgString := duration.PostgreSQLString()
			if pgString != config.expected {
				t.Errorf("Expected PostgreSQL string %s, got %s", config.expected, pgString)
			}
		})
	}
}
