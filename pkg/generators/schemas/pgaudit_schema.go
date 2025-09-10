package schemas

// GetPGAuditSchema returns the JSON schema definition for PGAudit configuration
func GetPGAuditSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"description":          "PGAudit extension configuration for PostgreSQL audit logging",
		"properties": map[string]interface{}{
			"log": map[string]interface{}{
				"type":        "string",
				"description": "Specifies which classes of statements will be logged by session audit logging",
				"enum":        []string{"none", "read", "write", "function", "role", "ddl", "misc", "misc_set", "all"},
				"default":     "none",
			},
			"log_catalog": map[string]interface{}{
				"type":        "string",
				"description": "Specifies that session logging should be enabled in the case where all relations in a statement are in pg_catalog",
				"enum":        []string{"on", "off"},
				"default":     "on",
			},
			"log_client": map[string]interface{}{
				"type":        "string",
				"description": "Specifies whether log messages will be visible to a client process",
				"enum":        []string{"on", "off"},
				"default":     "off",
			},
			"log_level": map[string]interface{}{
				"type":        "string",
				"description": "Specifies the log level that will be used for log entries",
				"enum":        []string{"debug5", "debug4", "debug3", "debug2", "debug1", "info", "notice", "warning", "log"},
				"default":     "log",
			},
			"log_parameter": map[string]interface{}{
				"type":        "string",
				"description": "Specifies that audit logging should include the parameters that were passed with the statement",
				"enum":        []string{"on", "off"},
				"default":     "off",
			},
			"log_parameter_max_size": map[string]interface{}{
				"type":        "string",
				"description": "Sets the maximum size of a parameter value that will be logged",
				"pattern":     "^[0-9]+[kMGT]?B?$",
				"default":     "0",
			},
			"log_relation": map[string]interface{}{
				"type":        "string",
				"description": "Specifies whether session audit logging should create a separate log entry for each relation referenced in a SELECT or DML statement",
				"enum":        []string{"on", "off"},
				"default":     "off",
			},
			"log_statement": map[string]interface{}{
				"type":        "string",
				"description": "Specifies whether logging will include the statement text and parameters (if enabled)",
				"enum":        []string{"on", "off"},
				"default":     "on",
			},
			"log_statement_once": map[string]interface{}{
				"type":        "string",
				"description": "Specifies whether logging will include the statement text and parameters (if enabled) with the first log entry for a statement/substatement combination or with every log entry",
				"enum":        []string{"on", "off"},
				"default":     "off",
			},
			"role": map[string]interface{}{
				"type":        "string",
				"description": "Specifies the master role to use for object audit logging",
			},
			"session_log_statement_name": map[string]interface{}{
				"type":        "string",
				"description": "Specifies whether the statement name, if provided, should be included in the session log",
				"enum":        []string{"on", "off"},
				"default":     "off",
			},
			// Object audit logging settings
			"object_log": map[string]interface{}{
				"type":        "string",
				"description": "Specifies which classes of statements will be logged by object audit logging",
				"enum":        []string{"none", "read", "write", "function", "role", "ddl", "misc", "misc_set", "all"},
				"default":     "none",
			},
			"object_log_catalog": map[string]interface{}{
				"type":        "string",
				"description": "Specifies that object logging should be enabled in the case where all relations in a statement are in pg_catalog",
				"enum":        []string{"on", "off"},
				"default":     "on",
			},
			// Performance and resource settings
			"max_stack_depth": map[string]interface{}{
				"type":        "string",
				"description": "Sets the maximum stack depth for audit logging to prevent infinite recursion",
				"pattern":     "^[0-9]+[kMGT]?B?$",
			},
			"filter_using_role": map[string]interface{}{
				"type":        "string",
				"description": "Specifies whether audit logging should be filtered using role-based access control",
				"enum":        []string{"on", "off"},
				"default":     "off",
			},
		},
	}
}