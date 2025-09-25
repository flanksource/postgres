package pkg

import "fmt"

// Core types needed by multiple packages

// PgVersion represents PostgreSQL version
type PgVersion string

// ExtensionInfo represents information about a PostgreSQL extension
type ExtensionInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Available bool   `json:"available,omitempty"`
}

// ValidationError provides detailed validation error information
type ValidationError struct {
	Line      int    // Line number if available
	Parameter string // Parameter name that failed
	Message   string // Error description
	Raw       string // Original error message
}

func (e *ValidationError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("line %d: %s: %s", e.Line, e.Parameter, e.Message)
	}
	if e.Parameter != "" {
		return fmt.Sprintf("%s: %s", e.Parameter, e.Message)
	}
	return e.Message
}

// Postgres interface for backward compatibility
// This allows existing code to work while server implementations are concrete
type Postgres interface {
	DescribeConfig() ([]interface{}, error)
	DetectVersion() (int, error)
	Health() error
	IsRunning() bool
	Start() error
	Stop() error
	Exists() bool
	GetVersion() PgVersion
	Validate(config []byte) error
}
