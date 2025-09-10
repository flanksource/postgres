package schemas

// GetPostgRESTSchema returns the JSON schema definition for PostgREST configuration
func GetPostgRESTSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"description":          "PostgREST API server configuration",
		"properties": map[string]interface{}{
			"db_uri": map[string]interface{}{
				"type":        "string",
				"description": "Database connection URI",
				"pattern":     "^postgres(ql)?://.*",
			},
			"db_schemas": map[string]interface{}{
				"type":        "string",
				"description": "Database schemas to expose via API",
				"default":     "public",
			},
			"db_pool": map[string]interface{}{
				"type":        "integer",
				"description": "Database connection pool size",
				"minimum":     1,
				"maximum":     1000,
				"default":     10,
			},
			"db_pool_timeout": map[string]interface{}{
				"type":        "integer",
				"description": "Database connection pool timeout in seconds",
				"minimum":     1,
				"default":     10,
			},
			"jwt_secret": map[string]interface{}{
				"type":        "string",
				"description": "JWT secret for authentication",
				"x-sensitive": true,
			},
			"jwt_aud": map[string]interface{}{
				"type":        "string",
				"description": "JWT audience claim",
				"default":     "",
			},
			"admin_role": map[string]interface{}{
				"type":        "string",
				"description": "Database role with admin privileges",
				"default":     "postgres",
			},
			"anonymous_role": map[string]interface{}{
				"type":        "string",
				"description": "Database role for anonymous access",
				"default":     "anon",
			},
			"server_host": map[string]interface{}{
				"type":        "string",
				"description": "Server host address",
				"default":     "0.0.0.0",
			},
			"server_port": map[string]interface{}{
				"type":        "integer",
				"description": "Server port",
				"minimum":     1,
				"maximum":     65535,
				"default":     3000,
			},
			"log_level": map[string]interface{}{
				"type":        "string",
				"description": "Logging level",
				"enum":        []string{"crit", "error", "warn", "info", "debug"},
				"default":     "error",
			},
			"pre_request": map[string]interface{}{
				"type":        "string",
				"description": "Pre-request function to call",
				"default":     "",
			},
			"server_ssl_cert": map[string]interface{}{
				"type":        "string",
				"description": "Path to SSL certificate file",
				"default":     "",
			},
			"server_ssl_key": map[string]interface{}{
				"type":        "string",
				"description": "Path to SSL private key file",
				"default":     "",
			},
			"max_rows": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum rows returned in a single response",
				"minimum":     1,
			},
			"role_claim_key": map[string]interface{}{
				"type":        "string",
				"description": "JWT claim key for role",
				"default":     "role",
			},
			"jwt_secret_is_base64": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether JWT secret is base64 encoded",
				"default":     false,
			},
		},
	}
}