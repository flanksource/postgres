package pkg

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/flanksource/postgres/pkg/installer"
	"github.com/flanksource/postgres/pkg/jwt"
	"github.com/flanksource/postgres/pkg/utils"
)

// PostgREST represents a PostgREST service instance
type PostgREST struct {
	Config *PostgrestConf
}

// NewPostgREST creates a new PostgREST service instance
func NewPostgREST(config *PostgrestConf) *PostgREST {
	return &PostgREST{
		Config: config,
	}
}

// Health performs a comprehensive health check of the PostgREST service
func (p *PostgREST) Health() error {
	if p == nil {
		return fmt.Errorf("PostgREST service is nil")
	}
	if p.Config == nil {
		return fmt.Errorf("PostgREST configuration not provided")
	}

	// Check if JWT secret is configured
	if p.Config.JwtSecret == nil || *p.Config.JwtSecret == "" {
		return fmt.Errorf("JWT secret not configured")
	}

	// Create JWT generator for health check
	jwtGenerator := jwt.NewJWTGenerator(utils.SensitiveString(*p.Config.JwtSecret), "", "")

	// Generate a health check token
	token, err := jwtGenerator.GenerateHealthCheckToken(p.Config.AdminRole)
	if err != nil {
		return fmt.Errorf("failed to generate health check JWT token: %w", err)
	}

	// Build the PostgREST URL
	url := fmt.Sprintf("http://%s:%d/", p.Config.ServerHost, p.Config.ServerPort)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Add authorization header with JWT token
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "postgres-health-check/1.0")

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("PostgREST health check request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("PostgREST returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Try to parse response as JSON to ensure it's valid
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read PostgREST response: %w", err)
	}

	var jsonResponse interface{}
	if err := json.Unmarshal(body, &jsonResponse); err != nil {
		return fmt.Errorf("PostgREST returned invalid JSON: %w", err)
	}

	return nil
}

// Start starts the PostgREST service (placeholder for future implementation)
func (p *PostgREST) Start() error {
	return fmt.Errorf("PostgREST start functionality not yet implemented")
}

// Stop stops the PostgREST service (placeholder for future implementation)
func (p *PostgREST) Stop() error {
	return fmt.Errorf("PostgREST stop functionality not yet implemented")
}

// GetOpenAPISchema retrieves the OpenAPI schema from PostgREST
func (p *PostgREST) GetOpenAPISchema() (map[string]interface{}, error) {
	if p.Config == nil {
		return nil, fmt.Errorf("PostgREST configuration not provided")
	}

	url := fmt.Sprintf("http://%s:%d/", p.Config.ServerHost, p.Config.ServerPort)

	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/openapi+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get OpenAPI schema: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PostgREST returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(body, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI schema: %w", err)
	}

	return schema, nil
}

// GetStatus returns detailed PostgREST service status
func (p *PostgREST) GetStatus() (*PostgRESTStatus, error) {
	status := &PostgRESTStatus{
		URL:           fmt.Sprintf("http://%s:%d", p.Config.ServerHost, p.Config.ServerPort),
		Healthy:       false,
		CheckTime:     time.Now(),
		AdminRole:     p.Config.AdminRole,
		AnonymousRole: p.Config.AnonymousRole,
	}

	// Perform health check
	if err := p.Health(); err != nil {
		status.Error = err.Error()
		return status, nil
	}

	status.Healthy = true
	return status, nil
}

// PostgRESTStatus represents the status of a PostgREST service
type PostgRESTStatus struct {
	URL           string    `json:"url"`
	Healthy       bool      `json:"healthy"`
	CheckTime     time.Time `json:"check_time"`
	AdminRole     string    `json:"admin_role"`
	AnonymousRole string    `json:"anonymous_role"`
	Error         string    `json:"error,omitempty"`
}

// Validate validates a PostgREST configuration file using the postgrest binary
func (p *PostgREST) Validate(config []byte) error {
	// Create a temporary file for the config
	tempFile, err := ioutil.TempFile("", "postgrest_validate_*.conf")
	if err != nil {
		return fmt.Errorf("failed to create temp config file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	// Write config to temp file
	if _, err := tempFile.Write(config); err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to write config to temp file: %w", err)
	}
	tempFile.Close()

	// First, perform basic syntax validation
	if err := p.validateConfigSyntax(config); err != nil {
		return err
	}

	// Try to validate using postgrest binary if available
	if err := p.validateWithBinary(tempFile.Name()); err != nil {
		return err
	}

	return nil
}

// validateConfigSyntax performs basic PostgREST configuration syntax validation
func (p *PostgREST) validateConfigSyntax(config []byte) error {
	lines := strings.Split(string(config), "\n")
	foundParams := make(map[string]bool)

	for lineNum, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Check for key=value pairs
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				return &ValidationError{
					Line:    lineNum + 1,
					Message: "invalid key=value format",
					Raw:     line,
				}
			}

			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			if key == "" {
				return &ValidationError{
					Line:      lineNum + 1,
					Parameter: key,
					Message:   "empty parameter name",
					Raw:       line,
				}
			}

			foundParams[key] = true

			// Validate required parameters
			if err := p.validateParameter(key, value, lineNum+1); err != nil {
				return err
			}

			continue
		}

		// If we get here, it's an invalid line
		return &ValidationError{
			Line:    lineNum + 1,
			Message: "invalid line format - expected key=value pair",
			Raw:     line,
		}
	}

	// Check for required parameters
	requiredParams := []string{"db-uri", "db-schema", "db-anon-role"}
	for _, param := range requiredParams {
		if !foundParams[param] {
			return &ValidationError{
				Parameter: param,
				Message:   "required parameter missing",
			}
		}
	}

	return nil
}

// validateParameter validates individual PostgREST parameters
func (p *PostgREST) validateParameter(key, value string, lineNum int) error {
	switch key {
	case "db-uri":
		if value == "" {
			return &ValidationError{
				Line:      lineNum,
				Parameter: key,
				Message:   "database URI cannot be empty",
			}
		}
		// Basic validation of PostgreSQL connection string
		if !strings.HasPrefix(value, "postgres://") && !strings.HasPrefix(value, "postgresql://") {
			return &ValidationError{
				Line:      lineNum,
				Parameter: key,
				Message:   "database URI must start with postgres:// or postgresql://",
				Raw:       value,
			}
		}

	case "db-schema":
		if value == "" {
			return &ValidationError{
				Line:      lineNum,
				Parameter: key,
				Message:   "database schema cannot be empty",
			}
		}
		// Validate schema name format (basic PostgreSQL identifier rules)
		if matched, _ := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_]*$`, value); !matched {
			return &ValidationError{
				Line:      lineNum,
				Parameter: key,
				Message:   "invalid schema name format",
				Raw:       value,
			}
		}

	case "db-anon-role":
		if value == "" {
			return &ValidationError{
				Line:      lineNum,
				Parameter: key,
				Message:   "anonymous role cannot be empty",
			}
		}
		// Validate role name format
		if matched, _ := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_]*$`, value); !matched {
			return &ValidationError{
				Line:      lineNum,
				Parameter: key,
				Message:   "invalid role name format",
				Raw:       value,
			}
		}

	case "server-host":
		if value != "" {
			// Validate host format (IP or hostname)
			if matched, _ := regexp.MatchString(`^[a-zA-Z0-9.-]+$`, value); !matched {
				return &ValidationError{
					Line:      lineNum,
					Parameter: key,
					Message:   "invalid host format",
					Raw:       value,
				}
			}
		}

	case "server-port":
		if value != "" {
			// Validate port number
			if matched, _ := regexp.MatchString(`^[0-9]+$`, value); !matched {
				return &ValidationError{
					Line:      lineNum,
					Parameter: key,
					Message:   "port must be a number",
					Raw:       value,
				}
			}
		}

	case "db-pool":
		if value != "" {
			// Validate pool size
			if matched, _ := regexp.MatchString(`^[0-9]+$`, value); !matched {
				return &ValidationError{
					Line:      lineNum,
					Parameter: key,
					Message:   "database pool size must be a number",
					Raw:       value,
				}
			}
		}

	case "max-rows":
		if value != "" {
			// Validate max rows
			if matched, _ := regexp.MatchString(`^[0-9]+$`, value); !matched {
				return &ValidationError{
					Line:      lineNum,
					Parameter: key,
					Message:   "max-rows must be a number",
					Raw:       value,
				}
			}
		}
	}

	return nil
}

// Install installs PostgREST binary with optional version and target directory
func (p *PostgREST) Install(version, targetDir string) error {
	inst := installer.New()
	return inst.InstallBinary("postgrest", version, targetDir)
}

// IsInstalled checks if PostgREST is installed in PATH
func (p *PostgREST) IsInstalled() bool {
	inst := installer.New()
	return inst.IsPostgRESTInstalled()
}

// InstalledVersion returns the installed PostgREST version
func (p *PostgREST) InstalledVersion() (string, error) {
	inst := installer.New()
	return inst.GetPostgRESTVersion()
}

// validateWithBinary attempts to validate using the postgrest binary
func (p *PostgREST) validateWithBinary(configPath string) error {
	// Try to run postgrest with --dump-config to validate configuration
	cmd := exec.Command("postgrest", configPath, "--dump-config")

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		return p.parsePostgRESTValidationError(outputStr, err)
	}

	// If postgrest --dump-config succeeds, the configuration is valid
	return nil
}

// parsePostgRESTValidationError parses PostgREST validation errors and returns structured error
func (p *PostgREST) parsePostgRESTValidationError(output string, originalErr error) error {
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse different error patterns
		if strings.Contains(line, "unknown configuration parameter") {
			// Example: unknown configuration parameter: invalid_param
			re := regexp.MustCompile(`unknown configuration parameter:?\s*([^\s]+)`)
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				return &ValidationError{
					Parameter: matches[1],
					Message:   "unknown configuration parameter",
					Raw:       line,
				}
			}
		}

		if strings.Contains(line, "invalid value") {
			// Example: invalid value for db-pool: invalid
			re := regexp.MustCompile(`invalid value for ([^:]+):\s*(.+)`)
			if matches := re.FindStringSubmatch(line); len(matches) > 2 {
				return &ValidationError{
					Parameter: strings.TrimSpace(matches[1]),
					Message:   fmt.Sprintf("invalid value: %s", strings.TrimSpace(matches[2])),
					Raw:       line,
				}
			}
		}

		if strings.Contains(line, "missing") && strings.Contains(line, "parameter") {
			// Example: missing required parameter: db-uri
			re := regexp.MustCompile(`missing.*parameter:?\s*([^\s]+)`)
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				return &ValidationError{
					Parameter: matches[1],
					Message:   "missing required parameter",
					Raw:       line,
				}
			}
		}

		if strings.Contains(line, "configuration error") || strings.Contains(line, "config error") {
			return &ValidationError{
				Message: "configuration contains errors",
				Raw:     line,
			}
		}

		// Check for database connection errors (indicates config issue)
		if strings.Contains(line, "connection to database") && strings.Contains(line, "failed") {
			return &ValidationError{
				Message: "database connection configuration error",
				Raw:     line,
			}
		}
	}

	// If we couldn't parse a specific error, return a general one
	return &ValidationError{
		Message: fmt.Sprintf("configuration validation failed: %s", strings.TrimSpace(output)),
		Raw:     output,
	}
}
