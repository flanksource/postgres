package pkg

import (
	"encoding/json"

	"github.com/flanksource/postgres/pkg/types"
)

// PostgresConf represents the PostgreSQL server configuration with the most commonly used settings.
// Additional properties can be set via the AdditionalProperties field.
type PostgresConf struct {
	// Memory Settings
	SharedBuffers      types.Size `json:"shared_buffers,omitempty" yaml:"shared_buffers,omitempty" jsonschema:"description=Sets the amount of memory the database server uses for shared memory buffers"`
	EffectiveCacheSize types.Size `json:"effective_cache_size,omitempty" yaml:"effective_cache_size,omitempty" jsonschema:"description=Sets the planner's assumption about the effective size of the disk cache"`
	WorkMem            types.Size `json:"work_mem,omitempty" yaml:"work_mem,omitempty" jsonschema:"description=Sets the amount of memory to be used by internal sort operations and hash tables"`
	MaintenanceWorkMem types.Size `json:"maintenance_work_mem,omitempty" yaml:"maintenance_work_mem,omitempty" jsonschema:"description=Sets the maximum memory to be used for maintenance operations"`
	WalBuffers         types.Size `json:"wal_buffers,omitempty" yaml:"wal_buffers,omitempty" jsonschema:"description=Sets the number of disk-page buffers in shared memory for WAL"`

	// Connection Settings
	MaxConnections     int    `json:"max_connections,omitempty" yaml:"max_connections,omitempty" jsonschema:"description=Sets the maximum number of concurrent connections,default=100,minimum=1"`
	ListenAddresses    string `json:"listen_addresses,omitempty" yaml:"listen_addresses,omitempty" jsonschema:"description=Sets the host name or IP address(es) on which the server listens for connections,default=localhost"`
	Port               int    `json:"port,omitempty" yaml:"port,omitempty" jsonschema:"description=Sets the TCP port the server listens on,default=5432,minimum=1,maximum=65535"`
	PasswordEncryption string `json:"password_encryption,omitempty" yaml:"password_encryption,omitempty" jsonschema:"description=Sets the algorithm for encrypting passwords,enum=md5,enum=scram-sha-256,default=scram-sha-256"`

	// WAL Settings
	WalLevel                   string         `json:"wal_level,omitempty" yaml:"wal_level,omitempty" jsonschema:"description=Sets the level of information written to the WAL,enum=minimal,enum=replica,enum=logical,default=replica"`
	MinWalSize                 types.Size     `json:"min_wal_size,omitempty" yaml:"min_wal_size,omitempty" jsonschema:"description=Sets the minimum size to shrink the WAL to"`
	MaxWalSize                 types.Size     `json:"max_wal_size,omitempty" yaml:"max_wal_size,omitempty" jsonschema:"description=Sets the maximum size to let the WAL grow to between automatic WAL checkpoints"`
	CheckpointCompletionTarget float64        `json:"checkpoint_completion_target,omitempty" yaml:"checkpoint_completion_target,omitempty" jsonschema:"description=Time spent flushing dirty buffers during checkpoint as fraction of checkpoint interval,default=0.9,minimum=0,maximum=1"`
	ArchiveMode                string         `json:"archive_mode,omitempty" yaml:"archive_mode,omitempty" jsonschema:"description=Allows archiving of WAL files,enum=off,enum=on,enum=always,default=off"`
	ArchiveCommand             string         `json:"archive_command,omitempty" yaml:"archive_command,omitempty" jsonschema:"description=Sets the shell command that will be called to archive a WAL file"`
	ArchiveTimeout             types.Duration `json:"archive_timeout,omitempty" yaml:"archive_timeout,omitempty" jsonschema:"description=Sets the amount of time to wait before forcing a switch to the next WAL file"`
	MaxWalSenders              int            `json:"max_wal_senders,omitempty" yaml:"max_wal_senders,omitempty" jsonschema:"description=Sets the maximum number of simultaneously running WAL sender processes,default=10"`

	// Query Optimization
	RandomPageCost          float64 `json:"random_page_cost,omitempty" yaml:"random_page_cost,omitempty" jsonschema:"description=Sets the planner's estimate of the cost of a nonsequentially fetched disk page,default=4.0"`
	EffectiveIoConcurrency  int     `json:"effective_io_concurrency,omitempty" yaml:"effective_io_concurrency,omitempty" jsonschema:"description=Number of simultaneous requests that can be handled efficiently by the disk subsystem,default=1"`
	DefaultStatisticsTarget int     `json:"default_statistics_target,omitempty" yaml:"default_statistics_target,omitempty" jsonschema:"description=Sets the default statistics target for table columns without a column-specific target,default=100"`

	// Parallel Processing
	MaxWorkerProcesses            int `json:"max_worker_processes,omitempty" yaml:"max_worker_processes,omitempty" jsonschema:"description=Sets the maximum number of background processes that the system can support,default=8"`
	MaxParallelWorkers            int `json:"max_parallel_workers,omitempty" yaml:"max_parallel_workers,omitempty" jsonschema:"description=Sets the maximum number of parallel workers that can be started by a single utility command,default=8"`
	MaxParallelWorkersPerGather   int `json:"max_parallel_workers_per_gather,omitempty" yaml:"max_parallel_workers_per_gather,omitempty" jsonschema:"description=Sets the maximum number of parallel processes per executor node,default=2"`
	MaxParallelMaintenanceWorkers int `json:"max_parallel_maintenance_workers,omitempty" yaml:"max_parallel_maintenance_workers,omitempty" jsonschema:"description=Sets the maximum number of parallel processes per maintenance operation,default=2"`

	// Logging
	LogStatement            string         `json:"log_statement,omitempty" yaml:"log_statement,omitempty" jsonschema:"description=Sets the type of statements logged,enum=none,enum=ddl,enum=mod,enum=all,default=none"`
	LogConnections          bool           `json:"log_connections,omitempty" yaml:"log_connections,omitempty" jsonschema:"description=Logs each successful connection,default=false"`
	LogDisconnections       bool           `json:"log_disconnections,omitempty" yaml:"log_disconnections,omitempty" jsonschema:"description=Logs end of a session including duration,default=false"`
	LogMinDurationStatement types.Duration `json:"log_min_duration_statement,omitempty" yaml:"log_min_duration_statement,omitempty" jsonschema:"description=Sets the minimum execution time above which statements will be logged (0 logs all statements)"`
	LogLinePrefix           string         `json:"log_line_prefix,omitempty" yaml:"log_line_prefix,omitempty" jsonschema:"description=Controls information prefixed to each log line,default=%m [%p]"`
	LogDestination          string         `json:"log_destination,omitempty" yaml:"log_destination,omitempty" jsonschema:"description=Sets the destination for server log output,enum=stderr,enum=csvlog,enum=syslog,enum=eventlog,default=stderr"`

	// SSL/Security
	SSL         bool   `json:"ssl,omitempty" yaml:"ssl,omitempty" jsonschema:"description=Enables SSL connections,default=false"`
	SSLCertFile string `json:"ssl_cert_file,omitempty" yaml:"ssl_cert_file,omitempty" jsonschema:"description=Location of the SSL server certificate file"`
	SSLKeyFile  string `json:"ssl_key_file,omitempty" yaml:"ssl_key_file,omitempty" jsonschema:"description=Location of the SSL server private key file"`

	// Autovacuum
	Autovacuum           bool           `json:"autovacuum,omitempty" yaml:"autovacuum,omitempty" jsonschema:"description=Starts the autovacuum subprocess,default=true"`
	AutovacuumMaxWorkers int            `json:"autovacuum_max_workers,omitempty" yaml:"autovacuum_max_workers,omitempty" jsonschema:"description=Sets the maximum number of simultaneously running autovacuum worker processes,default=3"`
	AutovacuumNaptime    types.Duration `json:"autovacuum_naptime,omitempty" yaml:"autovacuum_naptime,omitempty" jsonschema:"description=Time to sleep between autovacuum runs,default=1min"`

	// Extensions
	SharedPreloadLibraries string `json:"shared_preload_libraries,omitempty" yaml:"shared_preload_libraries,omitempty" jsonschema:"description=Lists shared libraries to preload into server"`

	// Huge Pages
	HugePages string `json:"huge_pages,omitempty" yaml:"huge_pages,omitempty" jsonschema:"description=Use of huge pages on Linux,enum=off,enum=on,enum=try,default=try"`

	// Allow additional PostgreSQL configuration parameters not explicitly defined above
	AdditionalProperties map[string]interface{} `json:"-" yaml:"-"`
}

// UnmarshalJSON implements custom JSON unmarshaling to handle additional properties
func (p *PostgresConf) UnmarshalJSON(data []byte) error {
	// Use a type alias to avoid infinite recursion
	type Alias PostgresConf
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(p),
	}

	// First unmarshal the known fields
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Then unmarshal everything to capture additional properties
	var allProps map[string]interface{}
	if err := json.Unmarshal(data, &allProps); err != nil {
		return err
	}

	// Remove known properties from the map
	knownProps := []string{
		"shared_buffers", "effective_cache_size", "work_mem", "maintenance_work_mem", "wal_buffers",
		"max_connections", "listen_addresses", "port", "password_encryption",
		"wal_level", "min_wal_size", "max_wal_size", "checkpoint_completion_target",
		"archive_mode", "archive_command", "archive_timeout", "max_wal_senders",
		"random_page_cost", "effective_io_concurrency", "default_statistics_target",
		"max_worker_processes", "max_parallel_workers", "max_parallel_workers_per_gather",
		"max_parallel_maintenance_workers", "log_statement", "log_connections",
		"log_disconnections", "log_min_duration_statement", "log_line_prefix", "log_destination",
		"ssl", "ssl_cert_file", "ssl_key_file", "autovacuum", "autovacuum_max_workers",
		"autovacuum_naptime", "shared_preload_libraries", "huge_pages",
	}

	for _, prop := range knownProps {
		delete(allProps, prop)
	}

	// Store remaining properties as additional
	if len(allProps) > 0 {
		p.AdditionalProperties = allProps
	}

	return nil
}

// MarshalJSON implements custom JSON marshaling to include additional properties
func (p PostgresConf) MarshalJSON() ([]byte, error) {
	// Use a type alias to avoid infinite recursion
	type Alias PostgresConf

	// Marshal the known fields
	data, err := json.Marshal(Alias(p))
	if err != nil {
		return nil, err
	}

	// If no additional properties, return as is
	if len(p.AdditionalProperties) == 0 {
		return data, nil
	}

	// Unmarshal to a map to merge additional properties
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	// Add additional properties
	for k, v := range p.AdditionalProperties {
		result[k] = v
	}

	// Marshal the combined result
	return json.Marshal(result)
}
