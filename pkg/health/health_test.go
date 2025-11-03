package health

import (
	"testing"
	"time"
)

// Mock implementations for testing
type mockPostgres struct {
	shouldFail bool
}

func (m *mockPostgres) SQL(query string, args ...any) ([]map[string]interface{}, error) {
	if m.shouldFail {
		return nil, &mockError{"mock postgres error"}
	}
	return []map[string]interface{}{{"result": 1}}, nil
}

type mockPgBouncer struct {
	shouldFail bool
}

func (m *mockPgBouncer) Health() error {
	if m.shouldFail {
		return &mockError{"mock pgbouncer error"}
	}
	return nil
}

type mockError struct {
	message string
}

func (m *mockError) Error() string {
	return m.message
}

func TestNewHealthChecker(t *testing.T) {
	config := &Config{
		DataDir:              "/tmp",
		DiskSpaceThreshold:   90.0,
		MemoryUsageThreshold: 80.0,
		CPUUsageThreshold:    90.0,
		WALSizeThreshold:     1024 * 1024 * 1024,
	}

	healthChecker, err := NewHealthChecker(config)
	if err != nil {
		t.Fatalf("Expected no error creating health checker, got: %v", err)
	}

	if healthChecker == nil {
		t.Fatal("Expected health checker to be created, got nil")
	}

	if healthChecker.h == nil {
		t.Fatal("Expected go-health instance to be created")
	}
}

func TestPostgreSQLChecker(t *testing.T) {
	tests := []struct {
		name       string
		postgres   *mockPostgres
		expectFail bool
	}{
		{
			name:       "healthy postgres",
			postgres:   &mockPostgres{shouldFail: false},
			expectFail: false,
		},
		{
			name:       "unhealthy postgres",
			postgres:   &mockPostgres{shouldFail: true},
			expectFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewPostgreSQLChecker(tt.postgres)
			status, err := checker.Status()

			if tt.expectFail {
				if err == nil {
					t.Error("Expected error for unhealthy postgres, got nil")
				}
				if status != "unhealthy" {
					t.Errorf("Expected status 'unhealthy', got %v", status)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for healthy postgres, got: %v", err)
				}
				if status != "healthy" {
					t.Errorf("Expected status 'healthy', got %v", status)
				}
			}
		})
	}
}

func TestPgBouncerChecker(t *testing.T) {
	tests := []struct {
		name       string
		pgbouncer  *mockPgBouncer
		expectFail bool
	}{
		{
			name:       "healthy pgbouncer",
			pgbouncer:  &mockPgBouncer{shouldFail: false},
			expectFail: false,
		},
		{
			name:       "unhealthy pgbouncer",
			pgbouncer:  &mockPgBouncer{shouldFail: true},
			expectFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewPgBouncerChecker(tt.pgbouncer)
			status, err := checker.Status()

			if tt.expectFail {
				if err == nil {
					t.Error("Expected error for unhealthy pgbouncer, got nil")
				}
				if status != "unhealthy" {
					t.Errorf("Expected status 'unhealthy', got %v", status)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for healthy pgbouncer, got: %v", err)
				}
				if status != "healthy" {
					t.Errorf("Expected status 'healthy', got %v", status)
				}
			}
		})
	}
}

func TestMemoryUsageChecker(t *testing.T) {
	checker := NewMemoryUsageChecker(95.0) // Very high threshold to ensure test passes
	status, err := checker.Status()

	if err != nil {
		t.Errorf("Expected no error for memory checker, got: %v", err)
	}

	statusMap, ok := status.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected status to be a map, got %T", status)
	}

	// Check required fields
	requiredFields := []string{"total_memory", "used_memory", "available_memory", "usage_percent", "threshold_percent", "go_allocated", "go_sys", "timestamp"}
	for _, field := range requiredFields {
		if _, exists := statusMap[field]; !exists {
			t.Errorf("Expected field %s in status, but not found", field)
		}
	}
}

func TestCPUUsageChecker(t *testing.T) {
	checker := NewCPUUsageChecker(95.0) // High threshold
	status, err := checker.Status()

	if err != nil {
		t.Errorf("Expected no error for CPU checker, got: %v", err)
	}

	statusMap, ok := status.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected status to be a map, got %T", status)
	}

	// Check required fields
	requiredFields := []string{"cpus", "goroutines", "threshold_percent", "timestamp"}
	for _, field := range requiredFields {
		if _, exists := statusMap[field]; !exists {
			t.Errorf("Expected field %s in status, but not found", field)
		}
	}
}

func TestHealthCheckerLifecycle(t *testing.T) {
	config := &Config{
		PostgresService:      &mockPostgres{shouldFail: false},
		DiskSpaceThreshold:   90.0,
		MemoryUsageThreshold: 80.0,
		CPUUsageThreshold:    90.0,
	}

	healthChecker, err := NewHealthChecker(config)
	if err != nil {
		t.Fatalf("Expected no error creating health checker, got: %v", err)
	}

	// Test Start
	if err := healthChecker.Start(); err != nil {
		t.Fatalf("Expected no error starting health checker, got: %v", err)
	}

	// Give it a moment to initialize
	time.Sleep(100 * time.Millisecond)

	// Test IsHealthy
	healthy := healthChecker.IsHealthy()
	if !healthy {
		t.Error("Expected health checker to be healthy")
	}

	// Test GetStatus
	status := healthChecker.GetStatus()
	if status == nil {
		t.Error("Expected status to be returned")
	}

	// Test Stop
	if err := healthChecker.Stop(); err != nil {
		t.Errorf("Expected no error stopping health checker, got: %v", err)
	}
}
