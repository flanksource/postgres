package schemas

// GetWalGSchema returns the JSON schema definition for WAL-G configuration
func GetWalGSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"description":          "WAL-G backup and archiving configuration",
		"properties": map[string]interface{}{
			"enabled": map[string]interface{}{
				"type":        "boolean",
				"description": "Enable or disable WAL-G",
				"default":     false,
			},
			"postgresql_data_dir": map[string]interface{}{
				"type":        "string",
				"description": "PostgreSQL data directory path",
				"default":     "/var/lib/postgresql/data",
			},
			// Storage configuration
			"s3_prefix": map[string]interface{}{
				"type":        "string",
				"description": "S3 storage prefix (e.g., s3://bucket/path/to/folder)",
				"pattern":     "^s3://.*",
			},
			"gs_prefix": map[string]interface{}{
				"type":        "string",
				"description": "Google Cloud Storage prefix (e.g., gs://bucket/path/to/folder)",
				"pattern":     "^gs://.*",
			},
			"az_prefix": map[string]interface{}{
				"type":        "string",
				"description": "Azure Storage prefix",
			},
			"file_prefix": map[string]interface{}{
				"type":        "string",
				"description": "Local file system prefix for backups",
			},
			// S3 configuration
			"s3_region": map[string]interface{}{
				"type":        "string",
				"description": "AWS S3 region",
				"default":     "us-east-1",
			},
			"s3_access_key": map[string]interface{}{
				"type":        "string",
				"description": "AWS S3 access key ID",
				"x-sensitive": true,
			},
			"s3_secret_key": map[string]interface{}{
				"type":        "string",
				"description": "AWS S3 secret access key",
				"x-sensitive": true,
			},
			"s3_session_token": map[string]interface{}{
				"type":        "string",
				"description": "AWS S3 session token (for temporary credentials)",
				"x-sensitive": true,
			},
			"s3_endpoint": map[string]interface{}{
				"type":        "string",
				"description": "Custom S3 endpoint URL",
			},
			"s3_use_ssl": map[string]interface{}{
				"type":        "boolean",
				"description": "Use SSL for S3 connections",
				"default":     true,
			},
			// Google Cloud configuration
			"gs_service_account_key": map[string]interface{}{
				"type":        "string",
				"description": "Google Cloud service account key JSON",
				"x-sensitive": true,
			},
			"gs_project_id": map[string]interface{}{
				"type":        "string",
				"description": "Google Cloud project ID",
			},
			// Azure configuration
			"az_account_name": map[string]interface{}{
				"type":        "string",
				"description": "Azure storage account name",
			},
			"az_account_key": map[string]interface{}{
				"type":        "string",
				"description": "Azure storage account key",
				"x-sensitive": true,
			},
			// Backup configuration
			"backup_schedule": map[string]interface{}{
				"type":        "string",
				"description": "Backup schedule in cron format",
				"default":     "0 2 * * *",
			},
			"backup_retain_count": map[string]interface{}{
				"type":        "integer",
				"description": "Number of backups to retain",
				"minimum":     1,
				"default":     7,
			},
			// Stream commands
			"stream_create_command": map[string]interface{}{
				"type":        "string",
				"description": "Command to create WAL stream",
			},
			"stream_restore_command": map[string]interface{}{
				"type":        "string",
				"description": "Command to restore from WAL stream",
			},
			// Compression settings
			"compression_method": map[string]interface{}{
				"type":        "string",
				"description": "Compression method for backups",
				"enum":        []string{"lz4", "lzma", "brotli", "zstd"},
				"default":     "lz4",
			},
			"compression_level": map[string]interface{}{
				"type":        "integer",
				"description": "Compression level (0-9)",
				"minimum":     0,
				"maximum":     9,
				"default":     1,
			},
			// WAL settings
			"wal_verify_checksum": map[string]interface{}{
				"type":        "boolean",
				"description": "Verify WAL checksums during backup",
				"default":     true,
			},
			"delta_max_steps": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum steps for delta backups",
				"minimum":     1,
				"default":     32,
			},
			// Upload settings
			"upload_concurrency": map[string]interface{}{
				"type":        "integer",
				"description": "Number of concurrent uploads",
				"minimum":     1,
				"maximum":     100,
				"default":     16,
			},
			"upload_disk_concurrency": map[string]interface{}{
				"type":        "integer",
				"description": "Number of concurrent disk operations",
				"minimum":     1,
				"maximum":     100,
				"default":     1,
			},
		},
	}
}