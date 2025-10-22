package schemas

import (
	"encoding/json"
	"fmt"
)

// SchemaType represents the type of schema
type SchemaType string

const (
	SchemaTypePgConfig  SchemaType = "pgconfig"
	SchemaTypePgBouncer SchemaType = "pgbouncer"
	SchemaTypePostgREST SchemaType = "postgrest"
	SchemaTypeWalG      SchemaType = "walg"
	SchemaTypePGAudit   SchemaType = "pgaudit"
	SchemaTypePgHBA     SchemaType = "pghba"
)

// GetSchemaJSON returns the raw JSON bytes for a given schema type
func GetSchemaJSON(schemaType SchemaType) ([]byte, error) {
	switch schemaType {
	case SchemaTypePgConfig:
		return GetPgconfigSchemaJSON(), nil
	case SchemaTypePgBouncer:
		return GetPgBouncerSchemaJSON(), nil
	case SchemaTypePostgREST:
		return GetPostgRESTSchemaJSON(), nil
	case SchemaTypeWalG:
		return GetWalGSchemaJSON(), nil
	case SchemaTypePGAudit:
		return GetPGAuditSchemaJSON(), nil
	case SchemaTypePgHBA:
		return GetPgHBASchemaJSON(), nil
	default:
		return nil, fmt.Errorf("unknown schema type: %s", schemaType)
	}
}

// GetSchema returns the parsed schema as a map for a given schema type
func GetSchema(schemaType SchemaType) (map[string]interface{}, error) {
	switch schemaType {
	case SchemaTypePgBouncer:
		return GetPgBouncerSchema()
	case SchemaTypePostgREST:
		return GetPostgRESTSchema()
	case SchemaTypeWalG:
		return GetWalGSchema()
	case SchemaTypePGAudit:
		return GetPGAuditSchema()
	case SchemaTypePgHBA:
		return GetPgHBASchema()
	case SchemaTypePgConfig:
		var schema map[string]interface{}
		if err := json.Unmarshal(GetPgconfigSchemaJSON(), &schema); err != nil {
			return nil, err
		}
		return schema, nil
	default:
		return nil, fmt.Errorf("unknown schema type: %s", schemaType)
	}
}

// ValidateAgainstSchema validates data against a schema
func ValidateAgainstSchema(data interface{}, schemaType SchemaType) error {
	schema, err := GetSchema(schemaType)
	if err != nil {
		return fmt.Errorf("failed to get schema: %w", err)
	}

	// Basic validation - in a real implementation you'd want to use a proper JSON schema validator
	_ = schema
	_ = data

	// For now, just return nil - proper validation would require a JSON schema library
	return nil
}

// GetAllSchemaTypes returns all available schema types
func GetAllSchemaTypes() []SchemaType {
	return []SchemaType{
		SchemaTypePgConfig,
		SchemaTypePgBouncer,
		SchemaTypePostgREST,
		SchemaTypeWalG,
		SchemaTypePGAudit,
		SchemaTypePgHBA,
	}
}
