package schemas

// GetPgBouncerSchema returns the JSON schema definition for PgBouncer configuration
func GetPgBouncerSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"description":          "PgBouncer connection pooler configuration",
		"properties": map[string]interface{}{
			"admin_password": map[string]interface{}{
				"type":        "string",
				"description": "Administrative password for PgBouncer",
			},
			"admin_user": map[string]interface{}{
				"type":        "string",
				"description": "Administrative user for PgBouncer",
			},
			"auth_type": map[string]interface{}{
				"type":        "string",
				"description": "Authentication type for PgBouncer",
				"enum":        []string{"any", "trust", "plain", "md5", "scram-sha-256", "cert", "hba", "pam"},
				"default":     "md5",
			},
			"default_pool_size": map[string]interface{}{
				"type":        "integer",
				"description": "Default pool size for databases",
				"minimum":     1,
				"default":     25,
			},
			"listen_address": map[string]interface{}{
				"type":        "string",
				"description": "Specifies the address to listen on",
				"default":     "0.0.0.0",
			},
			"listen_port": map[string]interface{}{
				"type":        "integer",
				"description": "Specifies the port to listen on",
				"minimum":     1,
				"maximum":     65535,
				"default":     6432,
			},
			"max_client_conn": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of client connections allowed",
				"minimum":     1,
				"default":     100,
			},
			"pool_mode": map[string]interface{}{
				"type":        "string",
				"description": "Pooling mode to use",
				"enum":        []string{"session", "transaction", "statement"},
				"default":     "transaction",
			},
			"server_reset_query": map[string]interface{}{
				"type":        "string",
				"description": "Query to run on server connection before returning to pool",
				"default":     "DISCARD ALL",
			},
			"auth_file": map[string]interface{}{
				"type":        "string",
				"description": "Path to authentication file",
				"default":     "userlist.txt",
			},
			"auth_query": map[string]interface{}{
				"type":        "string",
				"description": "Query to authenticate users",
				"default":     "SELECT usename, passwd FROM pg_shadow WHERE usename=$1",
			},
			"min_pool_size": map[string]interface{}{
				"type":        "integer",
				"description": "Minimum pool size",
				"minimum":     0,
				"default":     0,
			},
			"reserve_pool_size": map[string]interface{}{
				"type":        "integer",
				"description": "Reserved pool size",
				"minimum":     0,
			},
			"server_lifetime": map[string]interface{}{
				"type":        "string",
				"description": "Maximum lifetime of a server connection",
				"pattern":     "^[0-9]+(us|ms|s|min|h|d)?$",
				"default":     "3600s",
			},
			"server_idle_timeout": map[string]interface{}{
				"type":        "string",
				"description": "Maximum idle time for server connections",
				"pattern":     "^[0-9]+(us|ms|s|min|h|d)?$",
				"default":     "600s",
			},
			"query_timeout": map[string]interface{}{
				"type":        "string",
				"description": "Query timeout",
				"pattern":     "^[0-9]+(us|ms|s|min|h|d)?$",
				"default":     "0",
			},
			"client_idle_timeout": map[string]interface{}{
				"type":        "string",
				"description": "Maximum idle time for client connections",
				"pattern":     "^[0-9]+(us|ms|s|min|h|d)?$",
				"default":     "0",
			},
			"databases": map[string]interface{}{
				"type":        "object",
				"description": "Database connection configurations",
				"additionalProperties": map[string]interface{}{
					"$ref": "#/definitions/DatabaseConfig",
				},
			},
		},
	}
}

// GetDatabaseConfigSchema returns the JSON schema for database configuration
func GetDatabaseConfigSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"description":          "Database connection configuration for PgBouncer",
		"properties": map[string]interface{}{
			"dbname": map[string]interface{}{
				"type":        "string",
				"description": "Database name",
			},
			"host": map[string]interface{}{
				"type":        "string",
				"description": "Database host",
				"default":     "localhost",
			},
			"port": map[string]interface{}{
				"type":        "integer",
				"description": "Database port",
				"minimum":     1,
				"maximum":     65535,
				"default":     5432,
			},
			"user": map[string]interface{}{
				"type":        "string",
				"description": "Database user",
			},
			"password": map[string]interface{}{
				"type":        "string",
				"description": "Database password",
			},
			"pool_size": map[string]interface{}{
				"type":        "integer",
				"description": "Pool size for this database",
				"minimum":     1,
			},
			"connect_query": map[string]interface{}{
				"type":        "string",
				"description": "Query to run on new connections",
			},
		},
	}
}