package pkg

// PgBouncerConf represents PgBouncer configuration
type PgBouncerConf struct {
	Databases         map[string]DatabaseConfig `json:"databases,omitempty" yaml:"databases,omitempty" jsonschema:"description=Database connection configurations"`
	ListenAddr        string                    `json:"listen_addr,omitempty" yaml:"listen_addr,omitempty" jsonschema:"description=Specifies the address to listen on (deprecated use listen_address)"`
	ListenAddress     string                    `json:"listen_address,omitempty" yaml:"listen_address,omitempty" jsonschema:"description=Specifies the address to listen on,default=0.0.0.0"`
	ListenPort        int                       `json:"listen_port,omitempty" yaml:"listen_port,omitempty" jsonschema:"description=Specifies the port to listen on,default=6432,minimum=1,maximum=65535"`
	AuthType          string                    `json:"auth_type,omitempty" yaml:"auth_type,omitempty" jsonschema:"description=Authentication type for PgBouncer,enum=any,enum=trust,enum=plain,enum=md5,enum=scram-sha-256,enum=cert,enum=hba,enum=pam,default=md5"`
	AuthFile          string                    `json:"auth_file,omitempty" yaml:"auth_file,omitempty" jsonschema:"description=Path to authentication file,default=userlist.txt"`
	AuthQuery         string                    `json:"auth_query,omitempty" yaml:"auth_query,omitempty" jsonschema:"description=Query to authenticate users,default=SELECT usename\, passwd FROM pg_shadow WHERE usename=$1"`
	PoolMode          string                    `json:"pool_mode,omitempty" yaml:"pool_mode,omitempty" jsonschema:"description=Pooling mode to use,enum=session,enum=transaction,enum=statement,default=transaction"`
	MaxClientConn     int                       `json:"max_client_conn,omitempty" yaml:"max_client_conn,omitempty" jsonschema:"description=Maximum number of client connections allowed,default=100,minimum=1"`
	DefaultPoolSize   int                       `json:"default_pool_size,omitempty" yaml:"default_pool_size,omitempty" jsonschema:"description=Default pool size for databases,default=25,minimum=1"`
	MinPoolSize       int                       `json:"min_pool_size,omitempty" yaml:"min_pool_size,omitempty" jsonschema:"description=Minimum pool size,default=0,minimum=0"`
	ReservePoolSize   *int                      `json:"reserve_pool_size,omitempty" yaml:"reserve_pool_size,omitempty" jsonschema:"description=Reserved pool size,minimum=0"`
	ServerLifetime    string                    `json:"server_lifetime,omitempty" yaml:"server_lifetime,omitempty" jsonschema:"description=Server connection lifetime,pattern=^[0-9]+(us|ms|s|min|h|d)?$"`
	ServerIdleTimeout string                    `json:"server_idle_timeout,omitempty" yaml:"server_idle_timeout,omitempty" jsonschema:"description=Maximum idle time for server connections,pattern=^[0-9]+(us|ms|s|min|h|d)?$"`
	ClientIdleTimeout string                    `json:"client_idle_timeout,omitempty" yaml:"client_idle_timeout,omitempty" jsonschema:"description=Maximum idle time for client connections,default=0,pattern=^[0-9]+(us|ms|s|min|h|d)?$"`
	QueryTimeout      string                    `json:"query_timeout,omitempty" yaml:"query_timeout,omitempty" jsonschema:"description=Query timeout,default=0,pattern=^[0-9]+(us|ms|s|min|h|d)?$"`
	AdminUser         *string                   `json:"admin_user,omitempty" yaml:"admin_user,omitempty" jsonschema:"description=Administrative user for PgBouncer"`
	AdminPassword     *string                   `json:"admin_password,omitempty" yaml:"admin_password,omitempty" jsonschema:"description=Administrative password for PgBouncer"`
}

// DatabaseConfig represents database configuration for PgBouncer
type DatabaseConfig struct {
	Host         string  `json:"host,omitempty" yaml:"host,omitempty" jsonschema:"description=Database host address,default=localhost"`
	Port         int     `json:"port,omitempty" yaml:"port,omitempty" jsonschema:"description=Database port,default=5432,minimum=1,maximum=65535"`
	Dbname       *string `json:"dbname,omitempty" yaml:"dbname,omitempty" jsonschema:"description=Database name to connect to"`
	User         *string `json:"user,omitempty" yaml:"user,omitempty" jsonschema:"description=Username for database connection"`
	Password     *string `json:"password,omitempty" yaml:"password,omitempty" jsonschema:"description=Password for database connection"`
	PoolSize     *int    `json:"pool_size,omitempty" yaml:"pool_size,omitempty" jsonschema:"description=Maximum pool size for this database,minimum=0"`
	ConnectQuery *string `json:"connect_query,omitempty" yaml:"connect_query,omitempty" jsonschema:"description=Query to run on each new connection"`
}

// PostgrestConf represents PostgREST configuration
type PostgrestConf struct {
	DbUri         *string `json:"db_uri,omitempty" yaml:"db_uri,omitempty" jsonschema:"description=PostgreSQL connection string"`
	DbSchemas     *string `json:"db_schemas,omitempty" yaml:"db_schemas,omitempty" jsonschema:"description=Database schemas exposed to the API,default=public"`
	DbAnonRole    *string `json:"db_anon_role,omitempty" yaml:"db_anon_role,omitempty" jsonschema:"description=Database role for anonymous access"`
	DbPool        *int    `json:"db_pool,omitempty" yaml:"db_pool,omitempty" jsonschema:"description=Maximum number of connections in the pool (deprecated use db_pool_size),default=10,minimum=1"`
	DbPoolSize    *int    `json:"db_pool_size,omitempty" yaml:"db_pool_size,omitempty" jsonschema:"description=Maximum number of connections in the pool,default=10,minimum=1"`
	ServerHost    *string `json:"server_host,omitempty" yaml:"server_host,omitempty" jsonschema:"description=Host to bind the PostgREST server,default=0.0.0.0"`
	ServerPort    *int    `json:"server_port,omitempty" yaml:"server_port,omitempty" jsonschema:"description=Port to bind the PostgREST server,default=3000,minimum=1,maximum=65535"`
	JwtSecret     *string `json:"jwt_secret,omitempty" yaml:"jwt_secret,omitempty" jsonschema:"description=JWT secret for authentication"`
	JwtAud        *string `json:"jwt_aud,omitempty" yaml:"jwt_aud,omitempty" jsonschema:"description=JWT audience claim"`
	MaxRows       *int    `json:"max_rows,omitempty" yaml:"max_rows,omitempty" jsonschema:"description=Maximum number of rows in a response,minimum=1"`
	PreRequest    *string `json:"pre_request,omitempty" yaml:"pre_request,omitempty" jsonschema:"description=Function to call before each request"`
	RoleClaimKey  *string `json:"role_claim_key,omitempty" yaml:"role_claim_key,omitempty" jsonschema:"description=JWT claim to use for database role,default=.role"`
	AdminRole     string  `json:"admin_role,omitempty" yaml:"admin_role,omitempty" jsonschema:"description=Database role for admin access"`
	AnonymousRole string  `json:"anonymous_role,omitempty" yaml:"anonymous_role,omitempty" jsonschema:"description=Database role for unauthenticated access"`
	LogLevel      *string `json:"log_level,omitempty" yaml:"log_level,omitempty" jsonschema:"description=Log level for PostgREST,enum=crit,enum=error,enum=warn,enum=info,default=info"`
}

// WalgConf represents WAL-G backup configuration
type WalgConf struct {
	Enabled              bool    `json:"enabled,omitempty" yaml:"enabled,omitempty" jsonschema:"description=Enable WAL-G backup and recovery,default=false"`
	S3Bucket             *string `json:"s3_bucket,omitempty" yaml:"s3_bucket,omitempty" jsonschema:"description=S3 bucket name for backups"`
	S3Endpoint           *string `json:"s3_endpoint,omitempty" yaml:"s3_endpoint,omitempty" jsonschema:"description=S3 endpoint URL for S3-compatible storage"`
	S3AccessKeyId        *string `json:"s3_access_key_id,omitempty" yaml:"s3_access_key_id,omitempty" jsonschema:"description=S3 access key ID"`
	S3SecretAccessKey    *string `json:"s3_secret_access_key,omitempty" yaml:"s3_secret_access_key,omitempty" jsonschema:"description=S3 secret access key"`
	S3AccessKey          *string `json:"s3_access_key,omitempty" yaml:"s3_access_key,omitempty" jsonschema:"description=S3 access key (alternative to s3_access_key_id)"`
	S3SecretKey          *string `json:"s3_secret_key,omitempty" yaml:"s3_secret_key,omitempty" jsonschema:"description=S3 secret key (alternative to s3_secret_access_key)"`
	S3SessionToken       *string `json:"s3_session_token,omitempty" yaml:"s3_session_token,omitempty" jsonschema:"description=S3 session token for temporary credentials"`
	S3Region             string  `json:"s3_region,omitempty" yaml:"s3_region,omitempty" jsonschema:"description=S3 region,default=us-east-1"`
	S3UsePathStyle       *bool   `json:"s3_use_path_style,omitempty" yaml:"s3_use_path_style,omitempty" jsonschema:"description=Use path-style S3 URLs,default=false"`
	S3UseSsl             *bool   `json:"s3_use_ssl,omitempty" yaml:"s3_use_ssl,omitempty" jsonschema:"description=Use SSL for S3 connections,default=true"`
	S3Prefix             *string `json:"s3_prefix,omitempty" yaml:"s3_prefix,omitempty" jsonschema:"description=S3 path prefix for backups"`
	GsPrefix             *string `json:"gs_prefix,omitempty" yaml:"gs_prefix,omitempty" jsonschema:"description=Google Cloud Storage path prefix"`
	GsServiceAccountKey  *string `json:"gs_service_account_key,omitempty" yaml:"gs_service_account_key,omitempty" jsonschema:"description=Google Cloud service account key JSON"`
	AzPrefix             *string `json:"az_prefix,omitempty" yaml:"az_prefix,omitempty" jsonschema:"description=Azure Storage path prefix"`
	AzAccountName        *string `json:"az_account_name,omitempty" yaml:"az_account_name,omitempty" jsonschema:"description=Azure Storage account name"`
	AzAccountKey         *string `json:"az_account_key,omitempty" yaml:"az_account_key,omitempty" jsonschema:"description=Azure Storage account key"`
	FilePrefix           *string `json:"file_prefix,omitempty" yaml:"file_prefix,omitempty" jsonschema:"description=Local filesystem path prefix for backups"`
	CompressionMethod    *string `json:"compression_method,omitempty" yaml:"compression_method,omitempty" jsonschema:"description=Compression method for backups (deprecated use compression_type),enum=lz4,enum=lzo,enum=zstd,enum=brotli,default=lz4"`
	CompressionType      *string `json:"compression_type,omitempty" yaml:"compression_type,omitempty" jsonschema:"description=Compression type for backups,enum=lz4,enum=lzo,enum=zstd,enum=brotli,default=lz4"`
	DiskRateLimitBps     *int    `json:"disk_rate_limit_bps,omitempty" yaml:"disk_rate_limit_bps,omitempty" jsonschema:"description=Disk I/O rate limit in bytes per second,minimum=0"`
	NetworkRateLimitBps  *int    `json:"network_rate_limit_bps,omitempty" yaml:"network_rate_limit_bps,omitempty" jsonschema:"description=Network rate limit in bytes per second,minimum=0"`
	BackupSchedule       string  `json:"backup_schedule,omitempty" yaml:"backup_schedule,omitempty" jsonschema:"description=Backup schedule in cron format"`
	BackupRetainCount    int     `json:"backup_retain_count,omitempty" yaml:"backup_retain_count,omitempty" jsonschema:"description=Number of backups to retain,default=7,minimum=1"`
	RetentionPolicy      *string `json:"retention_policy,omitempty" yaml:"retention_policy,omitempty" jsonschema:"description=Retention policy configuration"`
	PostgresqlDataDir    string  `json:"postgresql_data_dir,omitempty" yaml:"postgresql_data_dir,omitempty" jsonschema:"description=PostgreSQL data directory path,default=/var/lib/postgresql/data"`
	PostgresqlPassword   *string `json:"postgresql_password,omitempty" yaml:"postgresql_password,omitempty" jsonschema:"description=PostgreSQL database password for WAL-G"`
	StreamCreateCommand  *string `json:"stream_create_command,omitempty" yaml:"stream_create_command,omitempty" jsonschema:"description=Command to create streaming backup"`
	StreamRestoreCommand *string `json:"stream_restore_command,omitempty" yaml:"stream_restore_command,omitempty" jsonschema:"description=Command to restore from streaming backup"`
}

// PgHBAEntry represents a pg_hba.conf rule element
type PgHBAEntry struct {
	Type     ConnectionType    `json:"type,omitempty" yaml:"type,omitempty"`
	Database string            `json:"database,omitempty" yaml:"database,omitempty"`
	User     string            `json:"user,omitempty" yaml:"user,omitempty"`
	Address  string            `json:"address,omitempty" yaml:"address,omitempty"`
	Method   PgAuthType        `json:"method,omitempty" yaml:"method,omitempty"`
	Options  map[string]string `json:"options,omitempty" yaml:"options,omitempty"`
}

// ConnectionType represents the connection type
type ConnectionType string

const (
	ConnectionTypeLocal ConnectionType = "local"
	ConnectionTypeHost  ConnectionType = "host"
	ConnectionTypeSSL   ConnectionType = "hostssl"
	ConnectionTypeNoSSL ConnectionType = "hostnossl"
)

// PgAuthType represents the authentication method
type PgAuthType string

const (
	AuthTrust    PgAuthType = "trust"
	AuthReject   PgAuthType = "reject"
	AuthMD5      PgAuthType = "md5"
	AuthPassword PgAuthType = "password"
	AuthScramSHA PgAuthType = "scram-sha-256"
	AuthPeer     PgAuthType = "peer"
	AuthIdent    PgAuthType = "ident"
	AuthCert     PgAuthType = "cert"
)

// PGAuditConf represents PGAudit extension configuration
type PGAuditConf struct {
	Log                 string  `json:"log,omitempty" yaml:"log,omitempty" jsonschema:"description=Statement classes to log,enum=none,enum=all,enum=read,enum=write,enum=function,enum=role,enum=ddl,enum=misc,enum=misc_set,default=all"`
	LogCatalog          string  `json:"log_catalog,omitempty" yaml:"log_catalog,omitempty" jsonschema:"description=Log statements for catalog objects,enum=on,enum=off,default=on"`
	LogClient           string  `json:"log_client,omitempty" yaml:"log_client,omitempty" jsonschema:"description=Log messages to client,enum=on,enum=off,default=off"`
	LogLevel            string  `json:"log_level,omitempty" yaml:"log_level,omitempty" jsonschema:"description=Log level for audit logs,enum=debug5,enum=debug4,enum=debug3,enum=debug2,enum=debug1,enum=info,enum=notice,enum=warning,enum=error,enum=log,enum=fatal,enum=panic,default=log"`
	LogParameter        string  `json:"log_parameter,omitempty" yaml:"log_parameter,omitempty" jsonschema:"description=Include parameters in audit logs,enum=on,enum=off,default=off"`
	LogParameterMaxSize string  `json:"log_parameter_max_size,omitempty" yaml:"log_parameter_max_size,omitempty" jsonschema:"description=Maximum parameter size to log,pattern=^\\d+(B|kB|KB|MB|GB)?$,default=0"`
	LogRelation         string  `json:"log_relation,omitempty" yaml:"log_relation,omitempty" jsonschema:"description=Create separate log entries per relation,enum=on,enum=off,default=off"`
	LogStatement        string  `json:"log_statement,omitempty" yaml:"log_statement,omitempty" jsonschema:"description=Include SQL statement text in logs,enum=on,enum=off,default=on"`
	LogStatementOnce    string  `json:"log_statement_once,omitempty" yaml:"log_statement_once,omitempty" jsonschema:"description=Log statement text only once,enum=on,enum=off,default=off"`
	Role                *string `json:"role,omitempty" yaml:"role,omitempty" jsonschema:"description=Database role to use for auditing"`
}

// PgconfigSchemaJson represents the main configuration schema
type PgconfigSchemaJson struct {
	Postgres  *PostgresConf  `json:"postgres,omitempty" yaml:"postgres,omitempty"`
	Pgbouncer *PgBouncerConf `json:"pgbouncer,omitempty" yaml:"pgbouncer,omitempty"`
	Postgrest *PostgrestConf `json:"postgrest,omitempty" yaml:"postgrest,omitempty"`
	Walg      *WalgConf      `json:"walg,omitempty" yaml:"walg,omitempty"`
	Pgaudit   *PGAuditConf   `json:"pgaudit,omitempty" yaml:"pgaudit,omitempty"`
}
