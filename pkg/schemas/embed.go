package schemas

import (
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
)

//go:embed data/pgconfig-schema.json
var pgconfigSchemaJSON []byte

// GetPgconfigSchemaJSON returns the raw JSON bytes for the main pgconfig schema
func GetPgconfigSchemaJSON() []byte {
	return pgconfigSchemaJSON
}

// GetPgBouncerSchemaJSON returns the raw JSON bytes for the PgBouncer schema
func GetPgBouncerSchemaJSON() []byte {
	data, err := os.ReadFile(filepath.Join("schema", "pgbouncer-schema.json"))
	if err != nil {
		panic("failed to read pgbouncer schema: " + err.Error())
	}
	return data
}

// GetPostgRESTSchemaJSON returns the raw JSON bytes for the PostgREST schema
func GetPostgRESTSchemaJSON() []byte {
	data, err := os.ReadFile(filepath.Join("schema", "postgrest-schema.json"))
	if err != nil {
		panic("failed to read postgrest schema: " + err.Error())
	}
	return data
}

// GetWalGSchemaJSON returns the raw JSON bytes for the WAL-G schema
func GetWalGSchemaJSON() []byte {
	data, err := os.ReadFile(filepath.Join("schema", "walg-schema.json"))
	if err != nil {
		panic("failed to read walg schema: " + err.Error())
	}
	return data
}

// GetPGAuditSchemaJSON returns the raw JSON bytes for the PGAudit schema
func GetPGAuditSchemaJSON() []byte {
	data, err := os.ReadFile(filepath.Join("schema", "pgaudit-schema.json"))
	if err != nil {
		panic("failed to read pgaudit schema: " + err.Error())
	}
	return data
}

// GetPgHBASchemaJSON returns the raw JSON bytes for the pg_hba.conf schema
func GetPgHBASchemaJSON() []byte {
	data, err := os.ReadFile(filepath.Join("schema", "pghba-schema.json"))
	if err != nil {
		panic("failed to read pghba schema: " + err.Error())
	}
	return data
}

// GetPgBouncerSchema returns the parsed PgBouncer schema as a map
func GetPgBouncerSchema() (map[string]interface{}, error) {
	var schema map[string]interface{}
	data := GetPgBouncerSchemaJSON()
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, err
	}
	return schema, nil
}

// GetPostgRESTSchema returns the parsed PostgREST schema as a map
func GetPostgRESTSchema() (map[string]interface{}, error) {
	var schema map[string]interface{}
	data := GetPostgRESTSchemaJSON()
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, err
	}
	return schema, nil
}

// GetWalGSchema returns the parsed WAL-G schema as a map
func GetWalGSchema() (map[string]interface{}, error) {
	var schema map[string]interface{}
	data := GetWalGSchemaJSON()
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, err
	}
	return schema, nil
}

// GetPGAuditSchema returns the parsed PGAudit schema as a map
func GetPGAuditSchema() (map[string]interface{}, error) {
	var schema map[string]interface{}
	data := GetPGAuditSchemaJSON()
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, err
	}
	return schema, nil
}

// GetPgHBASchema returns the parsed pg_hba.conf schema as a map
func GetPgHBASchema() (map[string]interface{}, error) {
	var schema map[string]interface{}
	data := GetPgHBASchemaJSON()
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, err
	}
	return schema, nil
}

// GetDatabaseConfigSchema returns the DatabaseConfig schema as a map
// This is typically a definition within the PgBouncer schema
func GetDatabaseConfigSchema() (map[string]interface{}, error) {
	// The DatabaseConfig is referenced in the PgBouncer schema
	// For now, return a simple database config schema
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
				"x-sensitive": true,
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
	}, nil
}