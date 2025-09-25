package health

import (
	"errors"
	"testing"
	"time"
)

func TestBaseHealthChecker_CreateStatusMap(t *testing.T) {
	base := &BaseHealthChecker{
		Name:        "test-checker",
		Description: "Test health checker",
		Threshold:   80,
		Interval:    5 * time.Second,
	}

	tests := []struct {
		name     string
		healthy  bool
		value    interface{}
		details  map[string]interface{}
		wantKeys []string
	}{
		{
			name:     "healthy status",
			healthy:  true,
			value:    75,
			details:  nil,
			wantKeys: []string{"name", "description", "timestamp", "status", "value", "threshold", "interval"},
		},
		{
			name:    "unhealthy status",
			healthy: false,
			value:   85,
			details: map[string]interface{}{
				"error": "threshold exceeded",
			},
			wantKeys: []string{"name", "description", "timestamp", "status", "value", "threshold", "interval", "error"},
		},
		{
			name:    "with additional details",
			healthy: true,
			value:   50,
			details: map[string]interface{}{
				"custom_field": "custom_value",
				"metric":       123,
			},
			wantKeys: []string{"name", "description", "timestamp", "status", "value", "threshold", "interval", "custom_field", "metric"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := base.CreateStatusMap(tt.healthy, tt.value, tt.details)

			// Check all expected keys are present
			for _, key := range tt.wantKeys {
				if _, ok := result[key]; !ok {
					t.Errorf("CreateStatusMap() missing key %s", key)
				}
			}

			// Check status value
			expectedStatus := "healthy"
			if !tt.healthy {
				expectedStatus = "unhealthy"
			}
			if result["status"] != expectedStatus {
				t.Errorf("CreateStatusMap() status = %v, want %v", result["status"], expectedStatus)
			}

			// Check base values
			if result["name"] != base.Name {
				t.Errorf("CreateStatusMap() name = %v, want %v", result["name"], base.Name)
			}
			if result["description"] != base.Description {
				t.Errorf("CreateStatusMap() description = %v, want %v", result["description"], base.Description)
			}
			if result["value"] != tt.value {
				t.Errorf("CreateStatusMap() value = %v, want %v", result["value"], tt.value)
			}
			if result["threshold"] != base.Threshold {
				t.Errorf("CreateStatusMap() threshold = %v, want %v", result["threshold"], base.Threshold)
			}
		})
	}
}

type mockChecker struct {
	*BaseHealthChecker
	checkFunc func() (interface{}, bool, error)
}

func (m *mockChecker) Status() (map[string]interface{}, error) {
	return PerformHealthCheck(m, m.checkFunc)
}

func (m *mockChecker) GetBase() *BaseHealthChecker {
	return m.BaseHealthChecker
}

func TestPerformHealthCheck(t *testing.T) {
	tests := []struct {
		name       string
		checkFunc  func() (interface{}, bool, error)
		wantError  bool
		wantStatus string
	}{
		{
			name: "successful healthy check",
			checkFunc: func() (interface{}, bool, error) {
				return 50, true, nil
			},
			wantError:  false,
			wantStatus: "healthy",
		},
		{
			name: "successful unhealthy check",
			checkFunc: func() (interface{}, bool, error) {
				return 90, false, nil
			},
			wantError:  false,
			wantStatus: "unhealthy",
		},
		{
			name: "check with error",
			checkFunc: func() (interface{}, bool, error) {
				return nil, false, errors.New("check failed")
			},
			wantError:  true,
			wantStatus: "unhealthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &mockChecker{
				BaseHealthChecker: &BaseHealthChecker{
					Name:        "mock-checker",
					Description: "Mock health checker",
					Threshold:   80,
				},
				checkFunc: tt.checkFunc,
			}

			status, err := checker.Status()

			if (err != nil) != tt.wantError {
				t.Errorf("PerformHealthCheck() error = %v, wantError %v", err, tt.wantError)
			}

			if status["status"] != tt.wantStatus {
				t.Errorf("PerformHealthCheck() status = %v, want %v", status["status"], tt.wantStatus)
			}

			if tt.wantError && status["error"] == nil {
				t.Errorf("PerformHealthCheck() expected error in status map")
			}
		})
	}
}

func TestComparisonFunctions(t *testing.T) {
	// Test GreaterThan
	if !GreaterThan(10, 5) {
		t.Error("GreaterThan(10, 5) = false, want true")
	}
	if GreaterThan(5, 10) {
		t.Error("GreaterThan(5, 10) = true, want false")
	}

	// Test LessThan
	if !LessThan(5, 10) {
		t.Error("LessThan(5, 10) = false, want true")
	}
	if LessThan(10, 5) {
		t.Error("LessThan(10, 5) = true, want false")
	}

	// Test GreaterThanOrEqual
	if !GreaterThanOrEqual(10, 10) {
		t.Error("GreaterThanOrEqual(10, 10) = false, want true")
	}
	if !GreaterThanOrEqual(11, 10) {
		t.Error("GreaterThanOrEqual(11, 10) = false, want true")
	}
	if GreaterThanOrEqual(9, 10) {
		t.Error("GreaterThanOrEqual(9, 10) = true, want false")
	}

	// Test LessThanOrEqual
	if !LessThanOrEqual(10, 10) {
		t.Error("LessThanOrEqual(10, 10) = false, want true")
	}
	if !LessThanOrEqual(9, 10) {
		t.Error("LessThanOrEqual(9, 10) = false, want true")
	}
	if LessThanOrEqual(11, 10) {
		t.Error("LessThanOrEqual(11, 10) = true, want false")
	}

	// Test CompareThreshold with float64
	if !CompareThreshold(75.5, 80.0, LessThan[float64]) {
		t.Error("CompareThreshold(75.5, 80.0, LessThan) = false, want true")
	}
}
