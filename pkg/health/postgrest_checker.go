package health

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/flanksource/postgres/pkg/jwt"
)

// PostgRESTChecker checks PostgREST API health using JWT authentication
type PostgRESTChecker struct {
	URL          string
	JWTGenerator *jwt.JWTGenerator
	AdminRole    string
	Timeout      time.Duration
}

// NewPostgRESTChecker creates a new PostgREST health checker
func NewPostgRESTChecker(url string, jwtGenerator *jwt.JWTGenerator, adminRole string, timeout time.Duration) *PostgRESTChecker {
	return &PostgRESTChecker{
		URL:          url,
		JWTGenerator: jwtGenerator,
		AdminRole:    adminRole,
		Timeout:      timeout,
	}
}

// Status implements the health.ICheckable interface
func (c *PostgRESTChecker) Status() (interface{}, error) {
	if c.JWTGenerator == nil {
		return nil, fmt.Errorf("JWT generator not configured")
	}

	// Generate a health check token
	token, err := c.JWTGenerator.GenerateHealthCheckToken(c.AdminRole)
	if err != nil {
		return nil, fmt.Errorf("failed to generate JWT token: %w", err)
	}

	// Create HTTP client with timeout
	client := &http.Client{Timeout: c.Timeout}

	// Try to access a basic endpoint that requires authentication
	req, err := http.NewRequest("GET", c.URL+"/", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("PostgREST request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("PostgREST returned status %d: %s", resp.StatusCode, string(body))
	}

	return map[string]interface{}{
		"status":    "healthy",
		"url":       c.URL,
		"auth_test": "passed",
		"timestamp": time.Now(),
	}, nil
}
