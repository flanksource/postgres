package generators

// FieldDescriptions contains all field descriptions copied from struct comments in model.go
var FieldDescriptions = map[string]string{
	// PostgresConf field descriptions
	"postgres.listen_addresses": "Specifies which IP address(es) to listen on\n" +
		"Default: \"localhost\" (local connections only)\n" +
		"Use \"*\" for all interfaces, \"0.0.0.0\" for all IPv4, \"::\" for all IPv6",

	"postgres.port": "Specifies the TCP port PostgreSQL listens on\n" +
		"Default: 5432",

	"postgres.max_connections": "Sets the maximum number of concurrent connections\n" +
		"Default: 100\n" +
		"Higher values require more shared memory",

	"postgres.shared_buffers": "Determines memory for caching data\n" +
		"Default: \"128MB\"\n" +
		"Recommended: 25% of system RAM for dedicated servers\n" +
		"Units: B, kB, MB, GB, TB (1024 multiplier)",

	"postgres.effective_cache_size": "Estimates OS and database buffer cache size\n" +
		"Default: \"4GB\"\n" +
		"Used by query planner, doesn't allocate memory\n" +
		"Recommended: 50-75% of system RAM",

	"postgres.maintenance_work_mem": "Sets memory for maintenance operations\n" +
		"Default: \"64MB\"\n" +
		"Used for VACUUM, CREATE INDEX, ALTER TABLE operations",

	"postgres.work_mem": "Sets memory for sort and hash operations\n" +
		"Default: \"4MB\"\n" +
		"Per operation, multiple operations can run simultaneously",

	"postgres.wal_level": "Determines information written to WAL\n" +
		"Default: \"replica\"\n" +
		"Values: \"minimal\", \"replica\", \"logical\"\n" +
		"\"replica\" enables standby servers, \"logical\" enables logical replication",

	"postgres.wal_buffers": "Sets memory for WAL data not yet written to disk\n" +
		"Default: \"16MB\"\n" +
		"Auto-sized to 1/32 of shared_buffers if not set",

	"postgres.checkpoint_completion_target": "Sets time to complete checkpoint\n" +
		"Default: 0.9 (90% of checkpoint interval)\n" +
		"Range: 0.0 to 1.0",

	"postgres.random_page_cost": "Sets cost estimate for random disk page fetches\n" +
		"Default: 4.0 (for traditional spinning disks)\n" +
		"Recommended: 1.1 for SSDs",

	"postgres.effective_io_concurrency": "Sets expected concurrent disk I/O operations\n" +
		"Default: 1\n" +
		"Recommended: 200+ for SSDs, depends on disk subsystem",

	"postgres.password_encryption": "Specifies password hashing method\n" +
		"Default: \"scram-sha-256\" (recommended)\n" +
		"Values: \"md5\", \"scram-sha-256\"",

	"postgres.superuser_password": "The password for the PostgreSQL superuser\n" +
		"Used for administrative tasks and initial setup",

	"postgres.ssl": "Enables SSL/TLS connections\n" +
		"Default: false\n" +
		"Requires SSL certificate and key files",

	"postgres.ssl_cert_file": "Specifies path to SSL server certificate\n" +
		"Default: \"server.crt\" (in data directory)\n" +
		"Must be PEM format",

	"postgres.ssl_key_file": "Specifies path to SSL server private key\n" +
		"Default: \"server.key\" (in data directory)\n" +
		"Must be PEM format, permissions must be 0600",

	"postgres.ssl_ca_file": "Specifies path to Certificate Authority file\n" +
		"Default: empty (no client certificate verification)\n" +
		"Used to verify client certificates",

	"postgres.ssl_crl_file": "Specifies path to Certificate Revocation List\n" +
		"Default: empty (no CRL checking)\n" +
		"Used to check revoked certificates",

	"postgres.ssl_ciphers": "Specifies allowed SSL cipher suites\n" +
		"Default: \"HIGH:MEDIUM:+3DES:!aNULL\" (secure ciphers)\n" +
		"OpenSSL cipher string format",

	"postgres.ssl_min_protocol_version": "Sets minimum TLS protocol version\n" +
		"Default: \"TLSv1.2\"\n" +
		"Values: \"TLSv1\", \"TLSv1.1\", \"TLSv1.2\", \"TLSv1.3\"",

	"postgres.ssl_max_protocol_version": "Sets maximum TLS protocol version\n" +
		"Default: empty (no maximum, use highest available)\n" +
		"Values: \"TLSv1\", \"TLSv1.1\", \"TLSv1.2\", \"TLSv1.3\"",

	"postgres.log_statement": "Controls which SQL statements are logged\n" +
		"Default: \"none\"\n" +
		"Values: \"none\", \"ddl\", \"mod\", \"all\"",

	"postgres.log_connections": "Enables logging of connection attempts\n" +
		"Default: false",

	"postgres.log_disconnections": "Enables logging of session terminations\n" +
		"Default: false",

	"postgres.statement_timeout": "Aborts statements taking longer than specified time\n" +
		"Default: \"0\" (disabled)\n" +
		"Units: us, ms, s, min, h, d",

	"postgres.lock_timeout": "Aborts statements waiting for lock longer than specified time\n" +
		"Default: \"0\" (disabled)\n" +
		"Units: us, ms, s, min, h, d",

	"postgres.idle_in_transaction_session_timeout": "Terminates idle in-transaction sessions\n" +
		"Default: \"0\" (disabled)\n" +
		"Units: us, ms, s, min, h, d",

	"postgres.shared_preload_libraries": "Specifies shared libraries to preload at startup\n" +
		"Default: empty (no preloaded libraries)\n" +
		"Comma-separated list of library names (e.g., \"pgaudit,pg_stat_statements\")",

	"postgres.include": "Specifies additional configuration files to include\n" +
		"Can be a single file path or comma-separated list\n" +
		"Default: empty (no includes)",

	"postgres.include_dir": "Specifies directory containing additional configuration files\n" +
		"All .conf files in the directory will be included\n" +
		"Default: empty (no include directory)",

	// PGAuditConf field descriptions
	"pgaudit.log": "Specifies which classes of statements will be logged by session audit logging\n" +
		"Classes: READ, WRITE, FUNCTION, ROLE, DDL, MISC, MISC_SET, ALL\n" +
		"Can use comma-separated list, can subtract classes with minus sign (e.g., \"ALL,-MISC\")\n" +
		"Default: \"none\"",

	"pgaudit.log_catalog": "Specifies that session logging should be enabled when all relations in a statement are in pg_catalog\n" +
		"When enabled, statements against catalog tables will be logged\n" +
		"Default: \"on\"",

	"pgaudit.log_client": "Specifies whether log messages will be visible to a client process\n" +
		"When enabled, audit messages are sent to the client in addition to the log file\n" +
		"Default: \"off\"",

	"pgaudit.log_level": "Specifies the log level for log entries\n" +
		"Valid values: DEBUG1-5, INFO, NOTICE, WARNING, LOG\n" +
		"Cannot use ERROR, FATAL, or PANIC levels\n" +
		"Default: \"log\"",

	"pgaudit.log_parameter": "Specifies that audit logging should include statement parameters\n" +
		"When enabled, parameters passed to prepared statements are logged\n" +
		"Default: \"off\"",

	"pgaudit.log_parameter_max_size": "Specifies maximum parameter value length for logging\n" +
		"Parameters longer than this will be truncated. 0 means no limit\n" +
		"Default: \"0\"",

	"pgaudit.log_relation": "Specifies whether to create separate log entries for each relation in a statement\n" +
		"When enabled, multi-table statements generate one log entry per table\n" +
		"Default: \"off\"",

	"pgaudit.log_rows": "Specifies that audit logging should include number of rows retrieved or affected\n" +
		"Shows row count for SELECT, INSERT, UPDATE, DELETE statements\n" +
		"Default: \"off\"",

	"pgaudit.log_statement": "Specifies whether logging will include statement text and parameters\n" +
		"When disabled, only basic audit information is logged\n" +
		"Default: \"on\"",

	"pgaudit.log_statement_once": "Specifies whether logging will include statement text only once\n" +
		"Reduces log volume for statements that affect multiple objects\n" +
		"Default: \"off\"",

	"pgaudit.role": "Specifies the role for object audit logging\n" +
		"When set, statements by this role or its members are logged\n" +
		"Default: empty (no role-based logging)",
}

// GetFieldDescription returns the description for a given field path
func GetFieldDescription(fieldPath string) string {
	if desc, exists := FieldDescriptions[fieldPath]; exists {
		return desc
	}
	return ""
}

// FormatConfigComment formats a field description for use in configuration files
func FormatConfigComment(fieldPath string, commentPrefix string) string {
	desc := GetFieldDescription(fieldPath)
	if desc == "" {
		return ""
	}

	lines := []string{}
	for _, line := range splitDescription(desc) {
		if line != "" {
			lines = append(lines, commentPrefix+" "+line)
		} else {
			lines = append(lines, commentPrefix)
		}
	}

	return joinLines(lines)
}

// splitDescription splits a multi-line description into individual lines
func splitDescription(desc string) []string {
	// Split on \n but preserve formatting
	return splitLines(desc)
}

// Helper functions
func splitLines(text string) []string {
	lines := []string{}
	current := ""

	for _, char := range text {
		if char == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(char)
		}
	}

	if current != "" {
		lines = append(lines, current)
	}

	return lines
}

func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}
