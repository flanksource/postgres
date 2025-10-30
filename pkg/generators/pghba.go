package generators

import (
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/postgres/pkg"
)

// PgHBAConfigGenerator generates pg_hba.conf configuration
type PgHBAConfigGenerator struct {
	Entries []pkg.PostgrestHBA
}

// NewPgHBAConfigGenerator creates a new pg_hba.conf configuration generator
func NewPgHBAConfigGenerator(authMethod pkg.PostgrestHBAMethod) *PgHBAConfigGenerator {

	generator := &PgHBAConfigGenerator{
		Entries: make([]pkg.PostgrestHBA, 0),
	}

	return generator
}

// AddEntry adds an authentication entry
func (g *PgHBAConfigGenerator) AddEntry(entry pkg.PostgrestHBA) {
	g.Entries = append(g.Entries, entry)
}

// AddHostEntry adds a TCP/IP connection entry
func (g *PgHBAConfigGenerator) AddSocketEntry(database, user string) {
	g.AddEntry(pkg.PostgrestHBA{
		Type:     pkg.ConnectionTypeLocal,
		Database: database,
		User:     user,
		Method:   pkg.AuthPeer,
	})
}

// AddHostEntry adds a TCP/IP connection entry
func (g *PgHBAConfigGenerator) AddHostEntry(database, user, address string, method pkg.PostgrestHBAMethod) {
	g.AddEntry(pkg.PostgrestHBA{
		Type:     pkg.ConnectionTypeHost,
		Database: database,
		User:     user,
		Address:  address,
		Method:   method,
	})
}

// AddSSLEntry adds an SSL-only connection entry
func (g *PgHBAConfigGenerator) AddSSLEntry(database, user, address string, method pkg.PostgrestHBAMethod) {
	g.AddEntry(pkg.PostgrestHBA{
		Type:     pkg.ConnectionTypeSSL,
		Database: database,
		User:     user,
		Address:  address,
		Method:   method,
	})
}

// GenerateConfigFile generates the actual pg_hba.conf file content
func (g *PgHBAConfigGenerator) GenerateConfigFile() string {
	var sb strings.Builder

	// File header
	sb.WriteString(g.generateHeader())

	// Configuration entries
	sb.WriteString(g.generateEntries())

	return sb.String()
}

func (g *PgHBAConfigGenerator) generateHeader() string {
	return fmt.Sprintf(`# PostgreSQL Client Authentication Configuration File
# ===================================================
#
# Generated automatically by postgres-cli
# Generated: %s
`,
		time.Now().Format("2006-01-02 15:04:05"),
	)
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

	parts = append(parts, fmt.Sprintf("%-23s", entry.Address))

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
