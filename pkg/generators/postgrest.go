package generators

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/postgres/pkg"
	"github.com/flanksource/postgres/pkg/pgtune"
	"github.com/flanksource/postgres/pkg/sysinfo"
)

// Helper functions for pointer conversions
func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

// PostgRESTConfigGenerator generates PostgREST configuration
type PostgRESTConfigGenerator struct {
	SystemInfo     *sysinfo.SystemInfo
	TunedParams    *pgtune.TunedParameters
	DatabaseName   string
	DatabaseHost   string
	DatabasePort   int
	DatabaseUser   string
	DatabasePass   string
	CustomSettings map[string]string
	config         *pkg.PostgrestConf // Store generated config for template use
}

// NewPostgRESTConfigGenerator creates a new PostgREST configuration generator
func NewPostgRESTConfigGenerator(sysInfo *sysinfo.SystemInfo, params *pgtune.TunedParameters) *PostgRESTConfigGenerator {
	return &PostgRESTConfigGenerator{
		SystemInfo:     sysInfo,
		TunedParams:    params,
		DatabaseName:   "postgres",
		DatabaseHost:   "localhost",
		DatabasePort:   5432,
		DatabaseUser:   "postgres",
		CustomSettings: make(map[string]string),
	}
}

// GenerateConfig generates a PostgREST configuration struct
func (g *PostgRESTConfigGenerator) GenerateConfig() (*pkg.PostgrestConf, error) {
	// Calculate connection pool size based on max_connections
	dbPool := calculatePostgRESTPoolSize(g.TunedParams.MaxConnections)

	// Generate database URI
	dbUri := g.generateDatabaseURI()

	// Generate JWT secret if not provided
	jwtSecret, err := g.generateJWTSecret()
	if err != nil {
		return nil, fmt.Errorf("failed to generate JWT secret: %w", err)
	}

	schemas := "public"
	config := &pkg.PostgrestConf{
		// Database connection
		DbUri:      &dbUri,
		DbSchemas:  &schemas,
		DbPoolSize: &dbPool,

		// JWT authentication
		JwtSecret:     &jwtSecret,
		AdminRole:     "postgres",
		AnonymousRole: "anon",

		// Server settings
		ServerHost: stringPtr("0.0.0.0"),
		ServerPort: intPtr(3000),
		LogLevel:   stringPtr("error"),
	}

	return config, nil
}

// GenerateConfigFile generates the actual PostgREST configuration file content
func (g *PostgRESTConfigGenerator) GenerateConfigFile() (string, error) {
	// Generate config first to use in templates
	config, err := g.GenerateConfig()
	if err != nil {
		return "", err
	}
	g.config = config

	var sb strings.Builder

	// File header
	sb.WriteString(g.generateHeader())

	// Database settings
	sb.WriteString(g.generateDatabaseSection())

	// JWT authentication
	sb.WriteString(g.generateAuthSection())

	// Server settings
	sb.WriteString(g.generateServerSection())

	// SSL settings
	sb.WriteString(g.generateSSLSection())

	// Custom settings
	if len(g.CustomSettings) > 0 {
		sb.WriteString(g.generateCustomSection())
	}

	return sb.String(), nil
}

// GenerateEnvFile generates a .env file for PostgREST
func (g *PostgRESTConfigGenerator) GenerateEnvFile() (string, error) {
	// Generate config first if not already generated
	if g.config == nil {
		config, err := g.GenerateConfig()
		if err != nil {
			return "", err
		}
		g.config = config
	}

	var sb strings.Builder

	// File header
	sb.WriteString(fmt.Sprintf(`# PostgREST Environment Configuration
# Generated automatically by PgTune
# System: %s
# Generated: %s
#
# This file contains environment variables for PostgREST configuration.
# Load this file or set these variables in your environment.
#

`,
		g.SystemInfo.OSType,
		time.Now().Format("2006-01-02 15:04:05"),
	))

	// Database connection
	sb.WriteString("# Database Connection\n")
	dbUri := ""
	if g.config.DbUri != nil {
		dbUri = *g.config.DbUri
	}
	sb.WriteString(fmt.Sprintf("DB_URI=\"%s\"\n", dbUri))
	if g.config.DbSchemas != nil {
		sb.WriteString(fmt.Sprintf("DB_SCHEMAS=\"%s\"\n", *g.config.DbSchemas))
	}
	if g.config.DbPoolSize != nil {
		sb.WriteString(fmt.Sprintf("DB_POOL=%d\n\n", *g.config.DbPoolSize))
	}

	// Authentication
	sb.WriteString("# JWT Authentication\n")
	jwtSecret := ""
	if g.config.JwtSecret != nil {
		jwtSecret = *g.config.JwtSecret
	}
	sb.WriteString(fmt.Sprintf("JWT_SECRET=\"%s\"\n", jwtSecret))
	if g.config.JwtAud != nil && *g.config.JwtAud != "" {
		sb.WriteString(fmt.Sprintf("JWT_AUD=\"%s\"\n", *g.config.JwtAud))
	} else {
		sb.WriteString("# JWT_AUD=\"your-app-name\"  # Optional: JWT audience\n")
	}
	sb.WriteString(fmt.Sprintf("ADMIN_ROLE=\"%s\"\n", g.config.AdminRole))
	sb.WriteString(fmt.Sprintf("ANONYMOUS_ROLE=\"%s\"\n\n", g.config.AnonymousRole))

	// Server settings
	sb.WriteString("# Server Configuration\n")
	if g.config.ServerHost != nil {
		sb.WriteString(fmt.Sprintf("SERVER_HOST=\"%s\"\n", *g.config.ServerHost))
	}
	if g.config.ServerPort != nil {
		sb.WriteString(fmt.Sprintf("SERVER_PORT=%d\n", *g.config.ServerPort))
	}
	if g.config.LogLevel != nil {
		sb.WriteString(fmt.Sprintf("LOG_LEVEL=\"%s\"\n\n", *g.config.LogLevel))
	}

	// SSL settings
	sb.WriteString("# SSL Configuration\n")
	// SSL configuration would go here if needed
	sb.WriteString("# SERVER_SSL_CERT=\"/path/to/server.crt\"\n")
	sb.WriteString("# SERVER_SSL_KEY=\"/path/to/server.key\"\n\n")

	// Custom settings
	if len(g.CustomSettings) > 0 {
		sb.WriteString("# Custom Settings\n")
		for key, value := range g.CustomSettings {
			sb.WriteString(fmt.Sprintf("%s=\"%s\"\n", strings.ToUpper(key), value))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

func (g *PostgRESTConfigGenerator) generateHeader() string {
	return fmt.Sprintf(`# PostgREST Configuration File
# Generated automatically by PgTune
# System: %s
# Generated: %s
#
# Memory: %.1f GB
# PostgreSQL Max Connections: %d
# PostgREST Pool Size: %d
#
# This configuration optimizes PostgREST for your system resources.
# Adjust settings as needed for your specific use case.
#

`,
		g.SystemInfo.OSType,
		time.Now().Format("2006-01-02 15:04:05"),
		g.SystemInfo.TotalMemoryGB(),
		g.TunedParams.MaxConnections,
		calculatePostgRESTPoolSize(g.TunedParams.MaxConnections),
	)
}

func (g *PostgRESTConfigGenerator) generateDatabaseSection() string {
	dbUri := ""
	if g.config.DbUri != nil {
		dbUri = *g.config.DbUri
	}
	schemas := "public"
	if g.config.DbSchemas != nil {
		schemas = *g.config.DbSchemas
	}
	poolSize := 10
	if g.config.DbPoolSize != nil {
		poolSize = *g.config.DbPoolSize
	}
	return fmt.Sprintf(`# Database Connection
db-uri = "%s"
db-schemas = "%s"
db-pool = %d

`,
		dbUri,
		schemas,
		poolSize,
	)
}

func (g *PostgRESTConfigGenerator) generateAuthSection() string {
	jwtSecret := ""
	if g.config.JwtSecret != nil {
		jwtSecret = *g.config.JwtSecret
	}
	authSection := fmt.Sprintf(`# JWT Authentication
jwt-secret = "%s"`, jwtSecret)

	if g.config.JwtAud != nil && *g.config.JwtAud != "" {
		authSection += fmt.Sprintf(`
jwt-aud = "%s"`, *g.config.JwtAud)
	} else {
		authSection += `
# jwt-aud = "your-app-name"  # Optional: JWT audience claim`
	}

	authSection += fmt.Sprintf(`
admin-role = "%s"
anonymous-role = "%s"

`, g.config.AdminRole, g.config.AnonymousRole)

	if g.config.PreRequest != nil && *g.config.PreRequest != "" {
		authSection += fmt.Sprintf(`# Pre-request function
pre-request = "%s"

`, *g.config.PreRequest)
	} else {
		authSection += `# Pre-request function (optional)
# pre-request = "public.check_auth"

`
	}

	return authSection
}

func (g *PostgRESTConfigGenerator) generateServerSection() string {
	host := "0.0.0.0"
	if g.config.ServerHost != nil {
		host = *g.config.ServerHost
	}
	port := 3000
	if g.config.ServerPort != nil {
		port = *g.config.ServerPort
	}
	logLevel := "error"
	if g.config.LogLevel != nil {
		logLevel = *g.config.LogLevel
	}
	return fmt.Sprintf(`# Server Configuration
server-host = "%s"
server-port = %d
log-level = "%s"

# Logging levels: crit, error, warn, info
# Use "info" for development, "error" for production

`, host, port, logLevel)
}

func (g *PostgRESTConfigGenerator) generateSSLSection() string {
	return `# SSL/TLS Configuration (uncomment to enable HTTPS)
# server-ssl-cert = "/path/to/server.crt"
# server-ssl-key = "/path/to/server.key"

# When SSL is enabled, PostgREST will serve HTTPS on the specified port
# Make sure the certificate files are readable by the PostgREST process

`
}

func (g *PostgRESTConfigGenerator) generateCustomSection() string {
	sb := strings.Builder{}
	sb.WriteString(`# Custom Settings
`)

	for key, value := range g.CustomSettings {
		sb.WriteString(fmt.Sprintf("%s = \"%s\"\n", key, value))
	}

	sb.WriteString("\n")
	return sb.String()
}

// generateDatabaseURI creates a PostgreSQL connection URI
func (g *PostgRESTConfigGenerator) generateDatabaseURI() string {
	uri := fmt.Sprintf("postgresql://%s", g.DatabaseUser)

	if g.DatabasePass != "" {
		uri += ":" + g.DatabasePass
	}

	uri += fmt.Sprintf("@%s:%d/%s", g.DatabaseHost, g.DatabasePort, g.DatabaseName)

	return uri
}

// generateJWTSecret generates a secure JWT secret
func (g *PostgRESTConfigGenerator) generateJWTSecret() (string, error) {
	// Generate 32 random bytes
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes for JWT secret: %w", err)
	}

	// Encode to base64
	return base64.StdEncoding.EncodeToString(bytes), nil
}

// SetDatabaseConfig sets the database connection parameters
func (g *PostgRESTConfigGenerator) SetDatabaseConfig(name, host string, port int, user, password string) {
	g.DatabaseName = name
	g.DatabaseHost = host
	g.DatabasePort = port
	g.DatabaseUser = user
	g.DatabasePass = password
}

// AddCustomSetting adds a custom setting to the configuration
func (g *PostgRESTConfigGenerator) AddCustomSetting(key, value string) {
	g.CustomSettings[key] = value
}

// calculatePostgRESTPoolSize calculates appropriate connection pool size for PostgREST
func calculatePostgRESTPoolSize(maxConnections int) int {
	// PostgREST typically needs fewer connections than the database max_connections
	// A good rule of thumb is 10-20% of max_connections
	poolSize := maxConnections / 10

	// Minimum pool size
	if poolSize < 5 {
		poolSize = 5
	}

	// Maximum pool size (don't use too many connections)
	if poolSize > 20 {
		poolSize = 20
	}

	return poolSize
}

// GenerateUserSetupSQL generates SQL commands to set up PostgREST users and roles
func (g *PostgRESTConfigGenerator) GenerateUserSetupSQL() string {
	return fmt.Sprintf(`-- PostgREST User Setup SQL
-- Generated automatically by PgTune
-- Run these commands as a PostgreSQL superuser

-- Create the anonymous role for unauthenticated requests
CREATE ROLE anon NOLOGIN;

-- Create the authenticator role (used by PostgREST to connect)
CREATE ROLE authenticator LOGIN PASSWORD 'your-password-here';

-- Grant the anonymous role to the authenticator
GRANT anon TO authenticator;

-- Grant usage on the public schema
GRANT USAGE ON SCHEMA public TO anon;

-- Example: Grant SELECT access on a table to anonymous users
-- GRANT SELECT ON public.your_table TO anon;

-- Create a role for authenticated users (optional)
CREATE ROLE authenticated NOLOGIN;
GRANT authenticated TO authenticator;
GRANT USAGE ON SCHEMA public TO authenticated;

-- Example: Grant more permissions to authenticated users
-- GRANT SELECT, INSERT, UPDATE, DELETE ON public.your_table TO authenticated;

-- Enable Row Level Security (RLS) on your tables for fine-grained access control
-- ALTER TABLE public.your_table ENABLE ROW LEVEL SECURITY;

-- Create policies for RLS (example)
-- CREATE POLICY "Allow anonymous read access" ON public.your_table
--   FOR SELECT TO anon USING (true);

-- Function to get current user role from JWT
CREATE OR REPLACE FUNCTION public.current_user_role()
RETURNS text AS $$
  SELECT current_setting('request.jwt.claims', true)::json->>'role';
$$ LANGUAGE sql STABLE;

-- Example pre-request function (optional)
CREATE OR REPLACE FUNCTION public.check_auth()
RETURNS void AS $$
BEGIN
  -- Add your custom authentication logic here
  -- This function is called before each request if specified in pre-request setting
  RETURN;
END;
$$ LANGUAGE plpgsql;
`)
}
