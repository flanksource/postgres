package server

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/flanksource/clicky"
)

// ListAvailableExtensions returns a list of available PostgreSQL extensions
func (p *Postgres) ListAvailableExtensions() ([]ExtensionInfo, error) {
	results, err := p.SQL("SELECT name, default_version FROM pg_available_extensions ORDER BY name;")
	if err != nil {
		return nil, fmt.Errorf("failed to list available extensions: %w", err)
	}

	var extensions []ExtensionInfo
	for _, row := range results {
		if nameVal, ok := row["name"]; ok {
			if versionVal, ok := row["default_version"]; ok {
				name := fmt.Sprintf("%v", nameVal)
				version := fmt.Sprintf("%v", versionVal)
				extensions = append(extensions, ExtensionInfo{
					Name:      name,
					Version:   version,
					Available: true,
				})
			}
		}
	}

	return extensions, nil
}

// GetSupportedExtensions returns the list of well-known supported extensions
func (p *Postgres) GetSupportedExtensions() []string {
	return []string{
		"pgvector",        // Vector similarity search
		"pgsodium",        // Modern cryptography
		"pgjwt",           // JSON Web Token support
		"pgaudit",         // PostgreSQL audit logging
		"pg_tle",          // Trusted Language Extensions
		"pg_stat_monitor", // Query performance monitoring
		"pg_repack",       // Table reorganization
		"pg_plan_filter",  // Query plan filtering
		"pg_net",          // Async networking
		"pg_jsonschema",   // JSON schema validation
		"pg_hashids",      // Short unique ID generation
		"pg_cron",         // Job scheduler
		"pg_safeupdate",   // Require WHERE clause in DELETE/UPDATE
		"index_advisor",   // Index recommendations
		"wal2json",        // WAL to JSON converter
	}
}

// InstallExtensions installs the specified PostgreSQL extensions
func (p *Postgres) InstallExtensions(extensions []string) error {
	if len(extensions) == 0 {
		return nil
	}

	// Extension mapping for special cases
	extensionMap := map[string]string{
		"pgvector":        "vector",
		"pgsodium":        "pgsodium",
		"pgjwt":           "pgjwt",
		"pgaudit":         "pgaudit",
		"pg_tle":          "pg_tle",
		"pg_stat_monitor": "pg_stat_monitor",
		"pg_repack":       "pg_repack",
		"pg_plan_filter":  "pg_plan_filter",
		"pg_net":          "pg_net",
		"pg_jsonschema":   "pg_jsonschema",
		"pg_hashids":      "pg_hashids",
		"pg_cron":         "pg_cron",
		"pg_safeupdate":   "safeupdate",
		"index_advisor":   "index_advisor",
		"wal2json":        "wal2json",
	}

	// Check if PostgreSQL is running by testing connectivity
	if !p.IsRunning() {
		return fmt.Errorf("PostgreSQL is not running")
	}

	for _, ext := range extensions {
		ext = strings.TrimSpace(ext)
		if ext == "" {
			continue
		}

		extName := extensionMap[ext]
		if extName == "" {
			extName = ext
		}

		if err := p.installSingleExtension(ext, extName); err != nil {
			return fmt.Errorf("failed to install extension %s: %w", ext, err)
		}
	}

	return nil
}

// installSingleExtension installs a single PostgreSQL extension with special handling
func (p *Postgres) installSingleExtension(originalName, extensionName string) error {
	psqlPath := filepath.Join(p.BinDir, "psql")
	dbName := "postgres"
	user := "postgres"
	host := "localhost"
	port := 5432

	// Use config values if available
	if p.Config != nil && p.Config.Port != 0 {
		port = p.Config.Port
	}

	// For localhost, generally no password needed with trust auth
	// No SuperuserPassword field available in PostgresConf

	switch originalName {
	case "pg_cron":
		// Install pg_cron with special permissions
		process := clicky.Exec(psqlPath, "-h", host, "-p", strconv.Itoa(port), "-U", user, "-d", dbName, "-c",
			"CREATE EXTENSION IF NOT EXISTS pg_cron CASCADE;").Run()

		if process.Err != nil {
			return fmt.Errorf("failed to create pg_cron extension: %w, output: %s", process.Err, process.Out())
		}

		// Grant usage on cron schema
		grantProcess := clicky.Exec(psqlPath, "-h", host, "-p", strconv.Itoa(port), "-U", user, "-d", dbName, "-c",
			"GRANT USAGE ON SCHEMA cron TO postgres;").Run()

		if grantProcess.Err != nil {
			// Non-fatal error for permission grant
			fmt.Printf("Warning: Failed to grant cron schema usage: %v\n", grantProcess.Err)
		}

	case "pgsodium":
		// Install pgsodium and create initial key
		process := clicky.Exec(psqlPath, "-h", host, "-p", strconv.Itoa(port), "-U", user, "-d", dbName, "-c",
			"CREATE EXTENSION IF NOT EXISTS pgsodium CASCADE;").Run()

		if process.Err != nil {
			return fmt.Errorf("failed to create pgsodium extension: %w, output: %s", process.Err, process.Out())
		}

		// Create pgsodium key
		keyProcess := clicky.Exec(psqlPath, "-h", host, "-p", strconv.Itoa(port), "-U", user, "-d", dbName, "-c",
			"SELECT pgsodium.create_key();").Run()

		if keyProcess.Err != nil {
			// Non-fatal error for key creation
			fmt.Printf("Warning: Failed to create pgsodium key: %v\n", keyProcess.Err)
		}

	default:
		// Standard extension installation
		process := clicky.Exec(psqlPath, "-h", host, "-p", strconv.Itoa(port), "-U", user, "-d", dbName, "-c",
			fmt.Sprintf("CREATE EXTENSION IF NOT EXISTS %s CASCADE;", extensionName)).Run()

		if process.Err != nil {
			return fmt.Errorf("failed to create extension %s: %w, output: %s", extensionName, process.Err, process.Out())
		}
	}

	return nil
}

// ListInstalledExtensions returns a list of installed PostgreSQL extensions
func (p *Postgres) ListInstalledExtensions() ([]ExtensionInfo, error) {
	results, err := p.SQL("SELECT extname, extversion FROM pg_extension WHERE extname NOT IN ('plpgsql') ORDER BY extname;")
	if err != nil {
		return nil, fmt.Errorf("failed to list installed extensions: %w", err)
	}

	var extensions []ExtensionInfo
	for _, row := range results {
		if nameVal, ok := row["extname"]; ok {
			if versionVal, ok := row["extversion"]; ok {
				name := fmt.Sprintf("%v", nameVal)
				version := fmt.Sprintf("%v", versionVal)
				extensions = append(extensions, ExtensionInfo{
					Name:    name,
					Version: version,
				})
			}
		}
	}

	return extensions, nil
}
