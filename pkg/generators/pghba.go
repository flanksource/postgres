package generators

import (
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/postgres/pkg"
	"github.com/flanksource/postgres/pkg/sysinfo"
)

// PgHBAConfigGenerator generates pg_hba.conf configuration
type PgHBAConfigGenerator struct {
	SystemInfo        *sysinfo.SystemInfo
	Entries           []pkg.PostgrestHBA
	DefaultAuthMethod string
}

// NewPgHBAConfigGenerator creates a new pg_hba.conf configuration generator
func NewPgHBAConfigGenerator(sysInfo *sysinfo.SystemInfo, defaultAuthMethod ...string) *PgHBAConfigGenerator {
	authMethod := "md5" // Default to md5 for backward compatibility
	if len(defaultAuthMethod) > 0 && defaultAuthMethod[0] != "" {
		authMethod = defaultAuthMethod[0]
	}

	generator := &PgHBAConfigGenerator{
		SystemInfo:        sysInfo,
		Entries:           make([]pkg.PostgrestHBA, 0),
		DefaultAuthMethod: authMethod,
	}

	// Add default entries
	generator.addDefaultEntries()

	return generator
}

// addDefaultEntries adds secure default authentication entries
func (g *PgHBAConfigGenerator) addDefaultEntries() {
	// Local connections for superuser - always use peer for postgres user
	g.AddEntry(pkg.PostgrestHBA{
		Type:     pkg.PostgrestHBAType("local"),
		Database: "all",
		User:     "postgres",
		Address:  nil,
		Method:   pkg.PostgrestHBAMethod("peer"),
		Options:  nil,
	})

	// Local connections for all users
	g.AddEntry(pkg.PostgrestHBA{
		Type:     pkg.PostgrestHBAType("local"),
		Database: "all",
		User:     "all",
		Address:  nil,
		Method:   pkg.PostgrestHBAMethod(g.DefaultAuthMethod),
		Options:  nil,
	})

	// IPv4 local connections
	addr1 := "127.0.0.1/32"
	g.AddEntry(pkg.PostgrestHBA{
		Type:     pkg.PostgrestHBAType("host"),
		Database: "all",
		User:     "all",
		Address:  &addr1,
		Method:   pkg.PostgrestHBAMethod(g.DefaultAuthMethod),
		Options:  nil,
	})

	// IPv6 local connections
	addr2 := "::1/128"
	g.AddEntry(pkg.PostgrestHBA{
		Type:     pkg.PostgrestHBAType("host"),
		Database: "all",
		User:     "all",
		Address:  &addr2,
		Method:   pkg.PostgrestHBAMethod(g.DefaultAuthMethod),
		Options:  nil,
	})

	// Private network connections (commented out by default)
	addr3 := "192.168.0.0/16"
	g.AddEntry(pkg.PostgrestHBA{
		Type:     pkg.PostgrestHBAType("host"),
		Database: "all",
		User:     "all",
		Address:  &addr3,
		Method:   pkg.PostgrestHBAMethod(g.DefaultAuthMethod),
		Options:  map[string]string{"comment": "COMMENTED"},
	})

	addr4 := "10.0.0.0/8"
	g.AddEntry(pkg.PostgrestHBA{
		Type:     pkg.PostgrestHBAType("host"),
		Database: "all",
		User:     "all",
		Address:  &addr4,
		Method:   pkg.PostgrestHBAMethod(g.DefaultAuthMethod),
		Options:  map[string]string{"comment": "COMMENTED"},
	})

	// SSL connections for remote access (commented out by default)
	addr5 := "0.0.0.0/0"
	g.AddEntry(pkg.PostgrestHBA{
		Type:     pkg.PostgrestHBAType("hostssl"),
		Database: "all",
		User:     "all",
		Address:  &addr5,
		Method:   pkg.PostgrestHBAMethod(g.DefaultAuthMethod),
		Options: map[string]string{
			"clientcert": "verify-ca",
			"comment":    "COMMENTED",
		},
	})
}

// AddEntry adds an authentication entry
func (g *PgHBAConfigGenerator) AddEntry(entry pkg.PostgrestHBA) {
	g.Entries = append(g.Entries, entry)
}

// SetDefaultAuthMethod updates the default authentication method
func (g *PgHBAConfigGenerator) SetDefaultAuthMethod(method string) {
	g.DefaultAuthMethod = method
}

// AddLocalEntry adds a local (Unix socket) connection entry
func (g *PgHBAConfigGenerator) AddLocalEntry(database, user, method string, options map[string]string) {
	g.AddEntry(pkg.PostgrestHBA{
		Type:     pkg.PostgrestHBAType("local"),
		Database: database,
		User:     user,
		Address:  nil,
		Method:   pkg.PostgrestHBAMethod(method),
		Options:  options,
	})
}

// AddHostEntry adds a TCP/IP connection entry
func (g *PgHBAConfigGenerator) AddHostEntry(database, user, address, method string, options map[string]string) {
	g.AddEntry(pkg.PostgrestHBA{
		Type:     pkg.PostgrestHBAType("host"),
		Database: database,
		User:     user,
		Address:  &address,
		Method:   pkg.PostgrestHBAMethod(method),
		Options:  options,
	})
}

// AddSSLEntry adds an SSL-only connection entry
func (g *PgHBAConfigGenerator) AddSSLEntry(database, user, address, method string, options map[string]string) {
	g.AddEntry(pkg.PostgrestHBA{
		Type:     pkg.PostgrestHBAType("hostssl"),
		Database: database,
		User:     user,
		Address:  &address,
		Method:   pkg.PostgrestHBAMethod(method),
		Options:  options,
	})
}

// AddNoSSLEntry adds a non-SSL connection entry
func (g *PgHBAConfigGenerator) AddNoSSLEntry(database, user, address, method string, options map[string]string) {
	g.AddEntry(pkg.PostgrestHBA{
		Type:     pkg.PostgrestHBAType("hostnossl"),
		Database: database,
		User:     user,
		Address:  &address,
		Method:   pkg.PostgrestHBAMethod(method),
		Options:  options,
	})
}

// GenerateConfigFile generates the actual pg_hba.conf file content
func (g *PgHBAConfigGenerator) GenerateConfigFile() string {
	var sb strings.Builder

	// File header
	sb.WriteString(g.generateHeader())

	// Authentication method documentation
	sb.WriteString(g.generateDocumentation())

	// Configuration entries
	sb.WriteString(g.generateEntries())

	return sb.String()
}

func (g *PgHBAConfigGenerator) generateHeader() string {
	return fmt.Sprintf(`# PostgreSQL Client Authentication Configuration File
# ===================================================
#
# Generated automatically by PgTune
# System: %s
# Generated: %s
#
# This file controls: which hosts are allowed to connect, how clients
# are authenticated, which PostgreSQL user names they can use, which
# databases they can access.
#
# Records take one of these forms:
#
# local         DATABASE  USER  METHOD  [OPTIONS]
# host          DATABASE  USER  ADDRESS  METHOD  [OPTIONS]
# hostssl       DATABASE  USER  ADDRESS  METHOD  [OPTIONS]
# hostnossl     DATABASE  USER  ADDRESS  METHOD  [OPTIONS]
#
# (The uppercase items must be replaced by actual values.)
#
# The first field is the connection type:
# - "local" is a Unix-domain socket
# - "host" is a TCP/IP socket (encrypted or not)
# - "hostssl" is a TCP/IP socket that is SSL-encrypted
# - "hostnossl" is a TCP/IP socket that is not SSL-encrypted
#
# DATABASE and USER can be:
# - "all" for all databases/users
# - a database or user name
# - a comma-separated list of names
# - a name prefixed with "+" for group names
#
# ADDRESS can be:
# - an IP address range in CIDR format (e.g., 192.168.1.0/24)
# - "all" for any address
# - "samehost" for connections from the same host
# - "samenet" for connections from the same subnet
#
# METHOD can be:
# - "trust" (no authentication required - NOT RECOMMENDED)
# - "reject" (reject the connection)
# - "md5" (widely compatible password authentication)
# - "scram-sha-256" (more secure, but requires newer clients)
# - "peer" (use OS user name, Unix sockets only)
# - "cert" (SSL certificate authentication)
#

`,
		g.SystemInfo.OSType,
		time.Now().Format("2006-01-02 15:04:05"),
	)
}

func (g *PgHBAConfigGenerator) generateDocumentation() string {
	return `# Security Recommendations:
# ========================
#
# 1. Use "md5" for password authentication (widely compatible)
# 2. Use "scram-sha-256" for enhanced security with newer clients
# 3. Use "hostssl" instead of "host" for remote connections
# 4. Specify specific IP ranges instead of "all" when possible
# 5. Use certificate authentication (clientcert=verify-ca) for sensitive data
# 6. Avoid "trust" method except for local development
# 7. Use "peer" authentication for local superuser connections
#
# Common scenarios:
# - Local development: "local all all trust"
# - Production local: "local all all peer" or "local all all md5"
# - Remote connections: "hostssl all all 0.0.0.0/0 md5 clientcert=verify-ca"
# - Application connections: "host dbname appuser 10.0.0.0/8 md5"
#

# TYPE  DATABASE        USER            ADDRESS                 METHOD

`
}

func (g *PgHBAConfigGenerator) generateEntries() string {
	var sb strings.Builder

	// Group entries by type for better organization
	localEntries := make([]pkg.PostgrestHBA, 0)
	hostEntries := make([]pkg.PostgrestHBA, 0)
	sslEntries := make([]pkg.PostgrestHBA, 0)
	nosslEntries := make([]pkg.PostgrestHBA, 0)

	for _, entry := range g.Entries {
		switch entry.Type {
		case "local":
			localEntries = append(localEntries, entry)
		case "host":
			hostEntries = append(hostEntries, entry)
		case "hostssl":
			sslEntries = append(sslEntries, entry)
		case "hostnossl":
			nosslEntries = append(nosslEntries, entry)
		}
	}

	// Local connections
	if len(localEntries) > 0 {
		sb.WriteString("# Local connections (Unix domain sockets)\n")
		for _, entry := range localEntries {
			sb.WriteString(g.formatEntry(entry))
		}
		sb.WriteString("\n")
	}

	// Host connections
	if len(hostEntries) > 0 {
		sb.WriteString("# TCP/IP connections (IPv4 and IPv6)\n")
		for _, entry := range hostEntries {
			sb.WriteString(g.formatEntry(entry))
		}
		sb.WriteString("\n")
	}

	// SSL connections
	if len(sslEntries) > 0 {
		sb.WriteString("# SSL-only connections\n")
		for _, entry := range sslEntries {
			sb.WriteString(g.formatEntry(entry))
		}
		sb.WriteString("\n")
	}

	// No-SSL connections
	if len(nosslEntries) > 0 {
		sb.WriteString("# Non-SSL connections\n")
		for _, entry := range nosslEntries {
			sb.WriteString(g.formatEntry(entry))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (g *PgHBAConfigGenerator) formatEntry(entry pkg.PostgrestHBA) string {
	var parts []string

	// Check if this entry is commented out
	commented := false
	if entry.Options != nil {
		if _, exists := entry.Options["comment"]; exists {
			commented = true
		}
	}

	commentPrefix := ""
	if commented {
		commentPrefix = "# "
	}

	// Type
	parts = append(parts, fmt.Sprintf("%-12s", entry.Type))

	// Database
	parts = append(parts, fmt.Sprintf("%-15s", entry.Database))

	// User
	parts = append(parts, fmt.Sprintf("%-15s", entry.User))

	// Address (only for non-local connections)
	if entry.Type != "local" && entry.Address != nil {
		parts = append(parts, fmt.Sprintf("%-23s", *entry.Address))
	} else if entry.Type != "local" {
		parts = append(parts, fmt.Sprintf("%-23s", ""))
	}

	// Method
	parts = append(parts, string(entry.Method))

	// Options
	var optionStrings []string
	if entry.Options != nil {
		for key, value := range entry.Options {
			if key == "comment" {
				continue // Skip the comment marker
			}
			if value == "" {
				optionStrings = append(optionStrings, key)
			} else {
				optionStrings = append(optionStrings, fmt.Sprintf("%s=%s", key, value))
			}
		}
	}

	result := commentPrefix + strings.Join(parts, " ")
	if len(optionStrings) > 0 {
		result += " " + strings.Join(optionStrings, " ")
	}

	result += "\n"

	return result
}

// AddDevelopmentEntries adds permissive entries for development environments
func (g *PgHBAConfigGenerator) AddDevelopmentEntries() {
	g.AddEntry(pkg.PostgrestHBA{
		Type:     pkg.PostgrestHBAType("local"),
		Database: "all",
		User:     "all",
		Address:  nil,
		Method:   pkg.PostgrestHBAMethod("trust"),
		Options:  nil,
	})

	addr1 := "127.0.0.1/32"
	g.AddEntry(pkg.PostgrestHBA{
		Type:     pkg.PostgrestHBAType("host"),
		Database: "all",
		User:     "all",
		Address:  &addr1,
		Method:   pkg.PostgrestHBAMethod("trust"),
		Options:  nil,
	})

	addr2 := "::1/128"
	g.AddEntry(pkg.PostgrestHBA{
		Type:     pkg.PostgrestHBAType("host"),
		Database: "all",
		User:     "all",
		Address:  &addr2,
		Method:   pkg.PostgrestHBAMethod("trust"),
		Options:  nil,
	})
}

// AddProductionSSLEntries adds secure SSL entries for production
func (g *PgHBAConfigGenerator) AddProductionSSLEntries(allowedNetworks []string) {
	for _, network := range allowedNetworks {
		networkAddr := network
		g.AddEntry(pkg.PostgrestHBA{
			Type:     pkg.PostgrestHBAType("hostssl"),
			Database: "all",
			User:     "all",
			Address:  &networkAddr,
			Method:   pkg.PostgrestHBAMethod("scram-sha-256"),
			Options: map[string]string{
				"clientcert": "verify-ca",
			},
		})
	}
}

// AddApplicationEntries adds entries for specific applications
func (g *PgHBAConfigGenerator) AddApplicationEntries(database, user, network string) {
	// SSL connection for the application
	g.AddEntry(pkg.PostgrestHBA{
		Type:     pkg.PostgrestHBAType("hostssl"),
		Database: database,
		User:     user,
		Address:  &network,
		Method:   pkg.PostgrestHBAMethod("scram-sha-256"),
		Options:  nil,
	})

	// Fallback non-SSL connection (commented out by default)
	g.AddEntry(pkg.PostgrestHBA{
		Type:     pkg.PostgrestHBAType("host"),
		Database: database,
		User:     user,
		Address:  &network,
		Method:   pkg.PostgrestHBAMethod("scram-sha-256"),
		Options:  map[string]string{"comment": "COMMENTED"},
	})
}

// ClearEntries removes all entries
func (g *PgHBAConfigGenerator) ClearEntries() {
	g.Entries = make([]pkg.PostgrestHBA, 0)
}
