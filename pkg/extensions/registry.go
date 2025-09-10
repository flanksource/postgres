package extensions

import (
	"fmt"
	"os"
	"strings"

	"github.com/flanksource/postgres/pkg"
)

// Extension represents a PostgreSQL extension configuration
type Extension struct {
	Name        string  `json:"name"`        // Extension package name
	SQLName     string  `json:"sql_name"`    // SQL name used in CREATE EXTENSION
	Description string  `json:"description"` // Brief description
	Required    bool    `json:"required"`    // Whether this extension is required
	MinVersion  float32 `json:"min_version"` // Minimum PostgreSQL version required
	Available   bool    `json:"available"`   // Whether extension files are available
	Installed   bool    `json:"installed"`   // Whether extension is installed in database

	Install     func(p *pkg.Postgres) error
	IsInstalled func(p *pkg.Postgres) bool
	Health      func(p *pkg.Postgres) error
}

// Registry holds the extension registry
type Registry struct {
	extensions map[string]*Extension
}

// NewRegistry creates a new extension registry
func NewRegistry() *Registry {
	return &Registry{
		extensions: make(map[string]*Extension),
	}
}

// GetDefaultRegistry returns the default extension registry with built-in extensions
func GetDefaultRegistry() *Registry {
	registry := NewRegistry()

	// Register built-in extensions with the same mapping as docker-entrypoint.sh
	registry.Register(&Extension{
		Name:        "pgvector",
		SQLName:     "vector",
		Description: "Open-source vector similarity search for PostgreSQL",
		Required:    false,
		MinVersion:  12.0,
	})

	registry.Register(&Extension{
		Name:        "pgsodium",
		SQLName:     "pgsodium",
		Description: "Modern cryptography for PostgreSQL using libsodium",
		Required:    false,
		MinVersion:  12.0,
	})

	registry.Register(&Extension{
		Name:        "pgjwt",
		SQLName:     "pgjwt",
		Description: "PostgreSQL implementation of JSON Web Tokens",
		Required:    false,
		MinVersion:  9.5,
	})

	registry.Register(&Extension{
		Name:        "pgaudit",
		SQLName:     "pgaudit",
		Description: "PostgreSQL audit logging extension",
		Required:    false,
		MinVersion:  9.5,
	})

	registry.Register(&Extension{
		Name:        "pg_tle",
		SQLName:     "pg_tle",
		Description: "Trusted Language Extensions for PostgreSQL",
		Required:    false,
		MinVersion:  13.0,
	})

	registry.Register(&Extension{
		Name:        "pg_stat_monitor",
		SQLName:     "pg_stat_monitor",
		Description: "Query performance monitoring tool for PostgreSQL",
		Required:    false,
		MinVersion:  11.0,
	})

	registry.Register(&Extension{
		Name:        "pg_repack",
		SQLName:     "pg_repack",
		Description: "Reorganize tables in PostgreSQL databases with minimal locks",
		Required:    false,
		MinVersion:  9.2,
	})

	registry.Register(&Extension{
		Name:        "pg_plan_filter",
		SQLName:     "pg_plan_filter",
		Description: "Planner hint extension for PostgreSQL",
		Required:    false,
		MinVersion:  10.0,
	})

	registry.Register(&Extension{
		Name:        "pg_net",
		SQLName:     "pg_net",
		Description: "Async HTTP/FTP client for PostgreSQL",
		Required:    false,
		MinVersion:  12.0,
	})

	registry.Register(&Extension{
		Name:        "pg_jsonschema",
		SQLName:     "pg_jsonschema",
		Description: "JSON schema validation for PostgreSQL",
		Required:    false,
		MinVersion:  12.0,
	})

	registry.Register(&Extension{
		Name:        "pg_hashids",
		SQLName:     "pg_hashids",
		Description: "Generate short unique ids from integers",
		Required:    false,
		MinVersion:  11.0,
	})

	registry.Register(&Extension{
		Name:        "pg_cron",
		SQLName:     "pg_cron",
		Description: "Simple cron-based job scheduler for PostgreSQL",
		Required:    false,
		MinVersion:  10.0,
	})

	registry.Register(&Extension{
		Name:        "pg_safeupdate",
		SQLName:     "safeupdate",
		Description: "Require WHERE clause in UPDATE and DELETE commands",
		Required:    false,
		MinVersion:  9.5,
	})

	registry.Register(&Extension{
		Name:        "index_advisor",
		SQLName:     "index_advisor",
		Description: "Query performance index recommendations",
		Required:    false,
		MinVersion:  12.0,
	})

	registry.Register(&Extension{
		Name:        "wal2json",
		SQLName:     "wal2json",
		Description: "JSON output plugin for logical decoding",
		Required:    false,
		MinVersion:  9.4,
	})

	registry.Register(&Extension{
		Name:        "hypopg",
		SQLName:     "hypopg",
		Description: "Hypothetical indexes for PostgreSQL",
		Required:    false,
		MinVersion:  9.2,
	})

	return registry
}

// Register adds an extension to the registry
func (r *Registry) Register(ext *Extension) {
	r.extensions[ext.Name] = ext
}

// Get retrieves an extension by name
func (r *Registry) Get(name string) (*Extension, bool) {
	ext, exists := r.extensions[name]
	return ext, exists
}

// GetBySQL retrieves an extension by its SQL name
func (r *Registry) GetBySQL(sqlName string) (*Extension, bool) {
	for _, ext := range r.extensions {
		if ext.SQLName == sqlName {
			return ext, true
		}
	}
	return nil, false
}

// List returns all registered extensions
func (r *Registry) List() []*Extension {
	extensions := make([]*Extension, 0, len(r.extensions))
	for _, ext := range r.extensions {
		extensions = append(extensions, ext)
	}
	return extensions
}

// ParseFromEnvironment parses the POSTGRES_EXTENSIONS environment variable
// and returns a list of extensions with their status updated
func (r *Registry) ParseFromEnvironment() ([]*Extension, error) {
	extensionsEnv := os.Getenv("POSTGRES_EXTENSIONS")
	if extensionsEnv == "" {
		return []*Extension{}, nil
	}

	extensionNames := strings.Split(extensionsEnv, ",")
	var result []*Extension

	for _, name := range extensionNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		ext, exists := r.Get(name)
		if !exists {
			// Create unknown extension entry
			ext = &Extension{
				Name:        name,
				SQLName:     name, // Default to same name
				Description: "Unknown extension",
				Required:    true, // Configured extensions are considered required
				MinVersion:  0,
			}
		} else {
			// Create a copy to avoid modifying the registry
			extCopy := *ext
			ext = &extCopy
			ext.Required = true // Configured extensions are required
		}

		// Update availability and installation status
		ext.Available = r.checkExtensionAvailable(ext.SQLName)
		ext.Installed = r.checkExtensionInstalled(ext.SQLName)

		result = append(result, ext)
	}

	return result, nil
}

// checkExtensionAvailable checks if extension files are available on the system
func (r *Registry) checkExtensionAvailable(sqlName string) bool {
	// Check for extension control file in standard PostgreSQL locations
	pgVersions := []int{17, 16, 15, 14} // Check multiple versions

	for _, version := range pgVersions {
		controlFile := fmt.Sprintf("/usr/share/postgresql/%d/extension/%s.control", version, sqlName)
		if _, err := os.Stat(controlFile); err == nil {
			return true
		}
	}

	return false
}

// checkExtensionInstalled checks if extension is installed in the database
func (r *Registry) checkExtensionInstalled(sqlName string) bool {
	// This is a simplified implementation
	// In a real scenario, this would connect to PostgreSQL and query pg_extension
	// For now, we assume if it's available, it might be installed
	return r.checkExtensionAvailable(sqlName)
}

// GetRequiredExtensions returns only the extensions marked as required
func (r *Registry) GetRequiredExtensions(extensions []*Extension) []*Extension {
	var required []*Extension
	for _, ext := range extensions {
		if ext.Required {
			required = append(required, ext)
		}
	}
	return required
}

// GetMissingExtensions returns extensions that are required but not available
func (r *Registry) GetMissingExtensions(extensions []*Extension) []*Extension {
	var missing []*Extension
	for _, ext := range extensions {
		if ext.Required && !ext.Available {
			missing = append(missing, ext)
		}
	}
	return missing
}

// GetUninstalledExtensions returns extensions that are available but not installed
func (r *Registry) GetUninstalledExtensions(extensions []*Extension) []*Extension {
	var uninstalled []*Extension
	for _, ext := range extensions {
		if ext.Available && !ext.Installed {
			uninstalled = append(uninstalled, ext)
		}
	}
	return uninstalled
}

// ValidatePostgreSQLVersion checks if extensions are compatible with PostgreSQL version
func (r *Registry) ValidatePostgreSQLVersion(extensions []*Extension, pgVersion float32) []string {
	var warnings []string
	for _, ext := range extensions {
		if ext.MinVersion > 0 && pgVersion < ext.MinVersion {
			warnings = append(warnings, fmt.Sprintf(
				"Extension %s requires PostgreSQL %.1f or higher (current: %.1f)",
				ext.Name, ext.MinVersion, pgVersion))
		}
	}
	return warnings
}
