package health

import (
	"time"
)

// BaseHealthChecker provides common functionality for all health checkers
type BaseHealthChecker struct {
	Name        string
	Description string
	Threshold   interface{}
	Interval    time.Duration
}

// CreateStatusMap creates a base status map with common fields
func (b *BaseHealthChecker) CreateStatusMap(healthy bool, value interface{}, details map[string]interface{}) map[string]interface{} {
	status := map[string]interface{}{
		"name":        b.Name,
		"description": b.Description,
		"timestamp":   time.Now().Unix(),
		"status":      "unknown",
		"value":       value,
	}

	if b.Threshold != nil {
		status["threshold"] = b.Threshold
	}

	if b.Interval > 0 {
		status["interval"] = b.Interval.String()
	}

	if healthy {
		status["status"] = "healthy"
	} else {
		status["status"] = "unhealthy"
	}

	// Add any additional details
	for k, v := range details {
		status[k] = v
	}

	return status
}

// CheckerWithBase is an interface for health checkers with base functionality
type CheckerWithBase interface {
	GetBase() *BaseHealthChecker
}

// PerformHealthCheck is a helper function that performs a health check using the base functionality
func PerformHealthCheck(checker CheckerWithBase, checkFunc func() (interface{}, bool, error)) (map[string]interface{}, error) {
	base := checker.GetBase()

	value, healthy, err := checkFunc()
	if err != nil {
		status := base.CreateStatusMap(false, value, map[string]interface{}{
			"error": err.Error(),
		})
		return status, err
	}

	status := base.CreateStatusMap(healthy, value, nil)
	return status, nil
}

// CompareThreshold is a generic helper for threshold comparisons
func CompareThreshold[T comparable](value, threshold T, compareFn func(T, T) bool) bool {
	return compareFn(value, threshold)
}

// GreaterThan returns true if a > b
func GreaterThan[T ~int | ~int64 | ~float64](a, b T) bool {
	return a > b
}

// LessThan returns true if a < b
func LessThan[T ~int | ~int64 | ~float64](a, b T) bool {
	return a < b
}

// GreaterThanOrEqual returns true if a >= b
func GreaterThanOrEqual[T ~int | ~int64 | ~float64](a, b T) bool {
	return a >= b
}

// LessThanOrEqual returns true if a <= b
func LessThanOrEqual[T ~int | ~int64 | ~float64](a, b T) bool {
	return a <= b
}
