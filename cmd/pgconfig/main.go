package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/flanksource/postgres/pkg"
	"github.com/flanksource/postgres/pkg/generators"
	"github.com/flanksource/postgres/pkg/installer"
	"github.com/flanksource/postgres/pkg/pgtune"
	"github.com/flanksource/postgres/pkg/server"
	"github.com/flanksource/postgres/pkg/sysinfo"
)

const (
	version = "1.0.0"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	switch command {
	case "generate":
		handleGenerateCommand()
	case "server":
		handleServer()
	case "validate":
		handleValidateCommand()
	case "supervisord":
		handleSupervisordCommand()
	case "install":
		handleInstallCommand()
	case "schema":
		handleSchemaCommand()
	case "version":
		fmt.Printf("pgconfig version %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`pgconfig - PostgreSQL Configuration Generator

Usage:
  pgconfig <command> [flags]

Commands:
  generate    Generate configuration files
  server      Start health check server
  validate    Validate configuration files
  supervisord Manage supervisord services
  install     Install binary tools (postgres, postgrest, wal-g)
  schema      Generate and manage JSON schemas from PostgreSQL
  version     Show version information
  help        Show this help message

Generate Command:
  pgconfig generate [config-type] [flags]

Config Types:
  conf        Generate only postgresql.conf
  pgbouncer   Generate only pgbouncer.ini
  postgrest   Generate only postgrest.conf and postgrest.env
  (none)      Generate all configuration files (default)

Flags:
  -c                  Configuration file path (env: PGCONFIG_CONFIG_FILE)
  --max-connections   Maximum number of connections (default: auto-detect)
  --db-type           Database type: web, oltp, dw, desktop, mixed (default: web)
  --output-dir        Output directory for config files (default: ./config)
  --memory            Override detected memory in GB (optional)
  --cpus              Override detected CPU count (optional)
  --pg-version        Override PostgreSQL version (default: from TARGET_VERSION env or 17.0)
  --validate          Validate generated configs before saving (default: false)

Environment Variables:
  PGCONFIG_CONFIG_FILE     Configuration file path
  PGCONFIG_MAX_CONNECTIONS Override max connections
  PGCONFIG_DB_TYPE         Override database type
  PGCONFIG_OUTPUT_DIR      Override output directory
  PGCONFIG_MEMORY_GB       Override detected memory in GB
  PGCONFIG_CPUS            Override detected CPU count
  PGCONFIG_PG_VERSION      Override PostgreSQL version
  PGCONFIG_VALIDATE        Enable config validation

Server Command:
  pgconfig server [flags]

Flags:
  --port              Server port (default: 8080)
  --config-dir        Directory to serve configs from (default: ./config)
  --max-connections   Maximum number of connections (default: auto-detect)
  --db-type          Database type: web, oltp, dw, desktop, mixed (default: web)

Environment Variables:
  PGCONFIG_PORT            Override server port
  PGCONFIG_CONFIG_DIR      Override config directory
  PGCONFIG_MAX_CONNECTIONS Override max connections
  PGCONFIG_DB_TYPE        Override database type

Validate Command:
  pgconfig validate <config-type> --file <path>

Config Types (for validation):
  postgres    Validate postgresql.conf file
  pgbouncer   Validate pgbouncer.ini file
  postgrest   Validate postgrest.conf file

Validate Flags:
  --file              Configuration file path to validate (required)

Examples:
  pgconfig generate --max-connections 200 --db-type web --output-dir /etc/postgresql
  pgconfig generate conf --max-connections 100 --db-type oltp
  pgconfig generate pgbouncer --output-dir ./config
  pgconfig generate -c config.yaml --output-dir ./config
  pgconfig server --port 3000 --max-connections 100 --db-type oltp

  pgconfig validate postgres --file /etc/postgresql/postgresql.conf
  pgconfig validate pgbouncer --file /etc/pgbouncer/pgbouncer.ini
  pgconfig validate postgrest --file /etc/postgrest/postgrest.conf

  pgconfig generate --validate --output-dir /tmp/configs
  pgconfig generate pgbouncer --validate

  # Using environment variables
  PGCONFIG_MEMORY_GB=8 PGCONFIG_CPUS=4 pgconfig generate conf
  PGCONFIG_PORT=3000 PGCONFIG_DB_TYPE=oltp pgconfig server

Install Command:
  pgconfig install [options]            Install binary tools based on configuration
  pgconfig install <binary> [options]   Install specific binary
  pgconfig install list                 List available binaries

Install Options:
  -c <config.yaml>    Install binaries based on config file (only enabled services)
  --version <version> Specify version to install (for single binary)
  --to <directory>    Target directory for installation

Available Binaries:
  postgres    PostgreSQL database server binaries (initdb, postgres, psql, etc.)
  postgrest   PostgREST API server for PostgreSQL databases
  wal-g       WAL-G backup and recovery tool for PostgreSQL

Install Examples:
  pgconfig install -c config.yaml                    Install enabled binaries from config
  pgconfig install -c config.yaml --to /opt/bin      Install to custom directory
  pgconfig install postgres                          Install PostgreSQL with default version
  pgconfig install postgres --version 15.5.0         Install specific PostgreSQL version
  pgconfig install postgrest                         Install PostgREST with default version
  pgconfig install wal-g --version v3.0.5            Install specific WAL-G version
  pgconfig install list                              Show all available binaries

Schema Command:
  pgconfig schema generate [flags]     Generate JSON schema from PostgreSQL parameters
  pgconfig schema validate [flags]     Validate existing schema against PostgreSQL
  pgconfig schema report [flags]       Generate parameter report

Schema Examples:
  pgconfig schema generate --version 16.1.0 --output schema-generated.json
  pgconfig schema validate --version 17.0.0
  pgconfig schema report --version 16.1.0 --output parameters.md
`)
}

func handleSchemaCommand() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Error: schema subcommand required\n")
		fmt.Fprintf(os.Stderr, "Usage: pgconfig schema <subcommand> [flags]\n")
		fmt.Fprintf(os.Stderr, "Subcommands:\n")
		fmt.Fprintf(os.Stderr, "  generate    Generate JSON schema from PostgreSQL parameters\n")
		fmt.Fprintf(os.Stderr, "  validate    Validate existing schema against PostgreSQL\n")
		fmt.Fprintf(os.Stderr, "  report      Generate parameter report\n")
		os.Exit(1)
	}

	subcommand := os.Args[2]
	
	switch subcommand {
	case "generate":
		if err := generateSchema(nil, os.Args[3:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "validate":
		if err := validateSchema(nil, os.Args[3:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "report":
		if err := generateReport(nil, os.Args[3:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Error: Unknown schema subcommand: %s\n", subcommand)
		fmt.Fprintf(os.Stderr, "Valid subcommands: generate, validate, report\n")
		os.Exit(1)
	}
}

func handleGenerateCommand() {
	// Check if there's a subcommand
	if len(os.Args) < 3 {
		// No subcommand provided, generate all configs
		handleGenerate("all")
		return
	}

	subcommand := os.Args[2]

	// Check if it's a flag (starts with -), then it's no subcommand
	if strings.HasPrefix(subcommand, "-") {
		handleGenerate("all")
		return
	}

	// Handle specific subcommands
	switch subcommand {
	case "conf":
		handleGenerate("conf")
	case "pgbouncer":
		handleGenerate("pgbouncer")
	case "postgrest":
		handleGenerate("postgrest")
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown config type: %s\n", subcommand)
		fmt.Fprintf(os.Stderr, "Valid config types: conf, pgbouncer, postgrest\n")
		os.Exit(1)
	}
}

func handleGenerate(configType string) {
	// Get values from environment variables first
	envMemoryGB := getEnvFloat("PGCONFIG_MEMORY_GB", 0)
	envCPUs := getEnvInt("PGCONFIG_CPUS", 0)
	envMaxConn := getEnvInt("PGCONFIG_MAX_CONNECTIONS", 0)
	envDBType := getEnvString("PGCONFIG_DB_TYPE", "web")
	envPGVersion := getEnvFloat("PGCONFIG_PG_VERSION", 0)
	envOutputDir := getEnvString("PGCONFIG_OUTPUT_DIR", "./config")

	envSave := getEnvString("PGCONFIG_SAVE", "false") == "true"
	envSkipExisting := getEnvString("PGCONFIG_SKIP_EXISTING", "false") == "true"
	envPGDataDir := getEnvString("PGCONFIG_PG_DATA_DIR", "")
	envValidate := getEnvString("PGCONFIG_VALIDATE", "false") == "true"
	envConfigFile := getEnvString("PGCONFIG_CONFIG_FILE", "")

	var (
		configFile     = flag.String("c", envConfigFile, "Configuration file path (env: PGCONFIG_CONFIG_FILE)")
		maxConnections = flag.Int("max-connections", envMaxConn, "Maximum number of connections (env: PGCONFIG_MAX_CONNECTIONS)")
		dbType         = flag.String("db-type", envDBType, "Database type: web, oltp, dw, desktop, mixed (env: PGCONFIG_DB_TYPE)")
		outputDir      = flag.String("output-dir", envOutputDir, "Output directory for config files (env: PGCONFIG_OUTPUT_DIR)")
		memoryGB       = flag.Float64("memory", envMemoryGB, "Override detected memory in GB (env: PGCONFIG_MEMORY_GB)")
		cpus           = flag.Int("cpus", envCPUs, "Override detected CPU count (env: PGCONFIG_CPUS)")
		pgVersion      = flag.Float64("pg-version", envPGVersion, "Override PostgreSQL version (env: PGCONFIG_PG_VERSION)")
		saveToSystem   = flag.Bool("save", envSave, "Save configs to system PostgreSQL directories (env: PGCONFIG_SAVE)")
		skipExisting   = flag.Bool("skip-existing", envSkipExisting, "Skip files that already exist (env: PGCONFIG_SKIP_EXISTING)")
		pgDataDir      = flag.String("pg-data-dir", envPGDataDir, "Override PostgreSQL data directory (env: PGCONFIG_PG_DATA_DIR)")
		validate       = flag.Bool("validate", envValidate, "Validate generated configs before saving (env: PGCONFIG_VALIDATE)")
	)

	// Parse flags, skipping the generate command and optional subcommand
	var flagsStart int
	if configType == "all" {
		flagsStart = 2 // "pgconfig generate"
	} else {
		flagsStart = 3 // "pgconfig generate <type>"
	}

	flag.CommandLine.Parse(os.Args[flagsStart:])

	// Load configuration from file if provided
	var loadedConf *pkg.Conf
	if *configFile != "" {
		var err error
		loadedConf, err = pkg.LoadConfig(*configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Loaded configuration from: %s\n", *configFile)
	}

	// Handle --save flag to determine system PostgreSQL directory
	if *saveToSystem {
		systemDir, err := detectPostgreSQLDataDir(*pgDataDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Could not detect PostgreSQL data directory: %v\n", err)
			fmt.Fprintf(os.Stderr, "Use --pg-data-dir to specify manually or remove --save flag\n")
			os.Exit(1)
		}
		*outputDir = systemDir
		fmt.Printf("Saving to PostgreSQL system directory: %s\n\n", systemDir)
	}

	fmt.Printf("Generating PostgreSQL configuration files...\n\n")

	// Detect system information
	sysInfo, err := sysinfo.DetectSystemInfo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not detect system info: %v\n", err)
		// Use defaults
		sysInfo = &sysinfo.SystemInfo{
			TotalMemoryBytes:  4 * 1024 * 1024 * 1024, // 4GB
			CPUCount:          4,
			OSType:            sysinfo.OSLinux,
			PostgreSQLVersion: 17.0,
			DiskType:          sysinfo.DiskSSD,
		}
	}

	// Apply overrides with validation
	if *memoryGB > 0 {
		if *memoryGB < 0.1 {
			fmt.Fprintf(os.Stderr, "Error: Memory must be at least 0.1 GB, got %.2f\n", *memoryGB)
			os.Exit(1)
		}
		if *memoryGB > 1024 {
			fmt.Fprintf(os.Stderr, "Warning: Very large memory specified: %.2f GB\n", *memoryGB)
		}
		oldMemoryGB := float64(sysInfo.TotalMemoryBytes) / (1024 * 1024 * 1024)
		sysInfo.TotalMemoryBytes = uint64(*memoryGB * 1024 * 1024 * 1024)
		fmt.Printf("Memory override: %.1f GB (was %.1f GB)\n", *memoryGB, oldMemoryGB)
	}
	if *cpus > 0 {
		if *cpus > 128 {
			fmt.Fprintf(os.Stderr, "Warning: Very high CPU count specified: %d\n", *cpus)
		}
		oldCPUs := sysInfo.CPUCount
		sysInfo.CPUCount = *cpus
		fmt.Printf("CPU override: %d (was %d)\n", *cpus, oldCPUs)
	}
	if *pgVersion > 0 {
		if *pgVersion < 9.0 || *pgVersion > 20.0 {
			fmt.Fprintf(os.Stderr, "Warning: Unusual PostgreSQL version specified: %.1f\n", *pgVersion)
		}
		oldVersion := sysInfo.PostgreSQLVersion
		sysInfo.PostgreSQLVersion = *pgVersion
		fmt.Printf("PostgreSQL version override: %.1f (was %.1f)\n", *pgVersion, oldVersion)
	}

	// Parse database type
	dbTypeEnum, err := parseDBType(*dbType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Calculate max connections if not provided
	if *maxConnections == 0 {
		*maxConnections = pgtune.GetRecommendedMaxConnections(dbTypeEnum)
	}

	fmt.Printf("System Information:\n")
	fmt.Printf("  Memory: %.1f GB\n", sysInfo.TotalMemoryGB())
	fmt.Printf("  CPUs: %d\n", sysInfo.CPUCount)
	fmt.Printf("  OS: %s\n", sysInfo.OSType)
	fmt.Printf("  PostgreSQL Version: %.1f\n", sysInfo.PostgreSQLVersion)
	fmt.Printf("  Disk Type: %s\n", sysInfo.DiskType)
	fmt.Printf("\nConfiguration:\n")
	fmt.Printf("  Max Connections: %d\n", *maxConnections)
	fmt.Printf("  Database Type: %s\n", *dbType)
	fmt.Printf("  Output Directory: %s\n", *outputDir)
	fmt.Printf("\n")

	// Generate tuned parameters
	tuningConfig := &pgtune.TuningConfig{
		SystemInfo:     sysInfo,
		MaxConnections: *maxConnections,
		DBType:         dbTypeEnum,
	}

	params, err := pgtune.CalculateOptimalConfig(tuningConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error calculating optimal config: %v\n", err)
		os.Exit(1)
	}

	// Create output directory
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Save actual files based on config type
	generatedFiles, err := saveSpecificConfigFiles(configType, *outputDir, sysInfo, params, *maxConnections, dbTypeEnum, *skipExisting, *validate, loadedConf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing config files: %v\n", err)
		os.Exit(1)
	}

	// Generate YAML configuration files
	yamlFiles, err := generateYAMLConfigFiles(*outputDir, loadedConf, *skipExisting)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating YAML config files: %v\n", err)
		os.Exit(1)
	}
	generatedFiles = append(generatedFiles, yamlFiles...)

	fmt.Printf("Configuration files generated successfully!\n\n")
	fmt.Printf("Generated files:\n")
	for _, file := range generatedFiles {
		fmt.Printf("  %s\n", file)
	}

	if len(params.Warnings) > 0 {
		fmt.Printf("\nWarnings:\n")
		for _, warning := range params.Warnings {
			fmt.Printf("  • %s\n", warning)
		}
	}
}

func handleServer() {
	// Get values from environment variables first
	envPort := getEnvInt("PGCONFIG_PORT", 8080)
	envConfigDir := getEnvString("PGCONFIG_CONFIG_DIR", "./config")
	envMaxConn := getEnvInt("PGCONFIG_MAX_CONNECTIONS", 0)
	envDBType := getEnvString("PGCONFIG_DB_TYPE", "web")

	var (
		port           = flag.Int("port", envPort, "Server port (env: PGCONFIG_PORT)")
		configDir      = flag.String("config-dir", envConfigDir, "Directory to serve configs from (env: PGCONFIG_CONFIG_DIR)")
		maxConnections = flag.Int("max-connections", envMaxConn, "Maximum number of connections (env: PGCONFIG_MAX_CONNECTIONS)")
		dbType         = flag.String("db-type", envDBType, "Database type: web, oltp, dw, desktop, mixed (env: PGCONFIG_DB_TYPE)")
	)

	flag.CommandLine.Parse(os.Args[2:])

	fmt.Printf("Starting PostgreSQL configuration health server...\n\n")

	// Parse database type
	dbTypeEnum, err := parseDBType(*dbType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Calculate max connections if not provided
	if *maxConnections == 0 {
		*maxConnections = pgtune.GetRecommendedMaxConnections(dbTypeEnum)
	}

	// Create and configure health server
	healthServer := server.NewHealthServer(*port, *configDir)

	if err := healthServer.ConfigFromFile(*maxConnections, dbTypeEnum); err != nil {
		fmt.Fprintf(os.Stderr, "Error configuring server: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Configuration:\n")
	fmt.Printf("  Port: %d\n", *port)
	fmt.Printf("  Config Directory: %s\n", *configDir)
	fmt.Printf("  Max Connections: %d\n", *maxConnections)
	fmt.Printf("  Database Type: %s\n", *dbType)
	fmt.Printf("\nEndpoints:\n")
	fmt.Printf("  http://localhost:%d/           - API documentation\n", *port)
	fmt.Printf("  http://localhost:%d/live       - Liveness check\n", *port)
	fmt.Printf("  http://localhost:%d/ready      - Readiness check\n", *port)
	fmt.Printf("  http://localhost:%d/info       - System information\n", *port)
	fmt.Printf("  http://localhost:%d/config     - Configuration summary\n", *port)
	fmt.Printf("  http://localhost:%d/health/status           - Detailed health status\n", *port)
	fmt.Printf("  http://localhost:%d/config/postgresql.conf  - PostgreSQL config\n", *port)
	fmt.Printf("  http://localhost:%d/config/pgbouncer.ini    - PgBouncer config\n", *port)
	fmt.Printf("  http://localhost:%d/config/postgrest.conf   - PostgREST config\n", *port)
	fmt.Printf("  http://localhost:%d/config/pg_hba.conf      - Authentication config\n", *port)
	fmt.Printf("\n")

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server in goroutine
	go func() {
		if err := healthServer.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			cancel()
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		fmt.Printf("\nShutting down server gracefully...\n")
	case <-ctx.Done():
		fmt.Printf("\nServer stopped due to error\n")
	}

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := healthServer.Stop(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "Error during shutdown: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Server stopped successfully\n")
}

func parseDBType(dbType string) (sysinfo.DBType, error) {
	switch dbType {
	case "web":
		return sysinfo.DBTypeWeb, nil
	case "oltp":
		return sysinfo.DBTypeOLTP, nil
	case "dw":
		return sysinfo.DBTypeDW, nil
	case "desktop":
		return sysinfo.DBTypeDesktop, nil
	case "mixed":
		return sysinfo.DBTypeMixed, nil
	default:
		return "", fmt.Errorf("invalid database type: %s (valid types: web, oltp, dw, desktop, mixed)", dbType)
	}
}

func saveSpecificConfigFiles(configType, outputDir string, sysInfo *sysinfo.SystemInfo, params *pgtune.TunedParameters, maxConnections int, dbType sysinfo.DBType, skipExisting bool, validate bool, loadedConf *pkg.Conf) ([]string, error) {
	var generatedFiles []string

	// Create generators
	pgGenerator := generators.NewPostgreSQLConfigGenerator(sysInfo, params)
	bouncerGenerator := generators.NewPgBouncerConfigGenerator(sysInfo, params)
	restGenerator := generators.NewPostgRESTConfigGenerator(sysInfo, params)
	hbaGenerator := generators.NewPgHBAConfigGenerator(sysInfo)

	// Configure extensions and PGAudit if loaded configuration is available
	if loadedConf != nil {
		if loadedConf.Extensions != nil {
			pgGenerator.SetExtensions(loadedConf.Extensions)
		}
		pgGenerator.SetPGAuditConf(loadedConf.Pgaudit)
	}

	switch configType {
	case "conf":
		// Generate only PostgreSQL config
		pgConfig := pgGenerator.GenerateConfigFile()

		// Validate if requested
		if validate {
			fmt.Printf("Validating postgresql.conf...\n")
			postgres := pkg.NewPostgres(nil, "")
			if err := postgres.Validate([]byte(pgConfig)); err != nil {
				return nil, fmt.Errorf("postgresql.conf validation failed: %w", err)
			}
			fmt.Printf("✅ postgresql.conf validation passed\n")
		}

		filename := filepath.Join(outputDir, "postgresql.conf")
		if err := writeFileWithSkip(filename, []byte(pgConfig), skipExisting); err != nil {
			return nil, fmt.Errorf("failed to write postgresql.conf: %w", err)
		}
		generatedFiles = append(generatedFiles, fmt.Sprintf("%s/postgresql.conf - PostgreSQL configuration", outputDir))

		// Generate PGAudit configuration if enabled
		if loadedConf != nil {
			pgauditGen := generators.NewPGAuditConfigGenerator(loadedConf.Pgaudit)
			if pgauditGen.IsEnabled() {
				pgauditConfig := pgauditGen.GenerateConfigFile()
				filename = filepath.Join(outputDir, "postgres.pgaudit.conf")
				if err := writeFileWithSkip(filename, []byte(pgauditConfig), skipExisting); err != nil {
					return nil, fmt.Errorf("failed to write postgres.pgaudit.conf: %w", err)
				}
				generatedFiles = append(generatedFiles, fmt.Sprintf("%s/postgres.pgaudit.conf - PGAudit extension configuration", outputDir))
			}
		}

	case "pgbouncer":
		// Generate only PgBouncer config
		bouncerConfig := bouncerGenerator.GenerateConfigFile()

		// Validate if requested
		if validate {
			fmt.Printf("Validating pgbouncer.ini...\n")
			pgbouncer := pkg.NewPgBouncer(nil)
			if err := pgbouncer.Validate([]byte(bouncerConfig)); err != nil {
				return nil, fmt.Errorf("pgbouncer.ini validation failed: %w", err)
			}
			fmt.Printf("✅ pgbouncer.ini validation passed\n")
		}

		filename := filepath.Join(outputDir, "pgbouncer.ini")
		if err := writeFileWithSkip(filename, []byte(bouncerConfig), skipExisting); err != nil {
			return nil, fmt.Errorf("failed to write pgbouncer.ini: %w", err)
		}
		generatedFiles = append(generatedFiles, fmt.Sprintf("%s/pgbouncer.ini - PgBouncer connection pooler configuration", outputDir))

	case "postgrest":
		// Generate PostgREST config and env file
		restConfig, err := restGenerator.GenerateConfigFile()
		if err != nil {
			return nil, fmt.Errorf("failed to generate postgrest.conf: %w", err)
		}

		// Validate if requested
		if validate {
			fmt.Printf("Validating postgrest.conf...\n")
			postgrest := pkg.NewPostgREST(nil)
			if err := postgrest.Validate([]byte(restConfig)); err != nil {
				return nil, fmt.Errorf("postgrest.conf validation failed: %w", err)
			}
			fmt.Printf("✅ postgrest.conf validation passed\n")
		}

		filename := filepath.Join(outputDir, "postgrest.conf")
		if err := writeFileWithSkip(filename, []byte(restConfig), skipExisting); err != nil {
			return nil, fmt.Errorf("failed to write postgrest.conf: %w", err)
		}
		generatedFiles = append(generatedFiles, fmt.Sprintf("%s/postgrest.conf - PostgREST API configuration", outputDir))

		envConfig, err := restGenerator.GenerateEnvFile()
		if err != nil {
			return nil, fmt.Errorf("failed to generate postgrest.env: %w", err)
		}
		filename = filepath.Join(outputDir, "postgrest.env")
		if err := writeFileWithSkip(filename, []byte(envConfig), skipExisting); err != nil {
			return nil, fmt.Errorf("failed to write postgrest.env: %w", err)
		}
		generatedFiles = append(generatedFiles, fmt.Sprintf("%s/postgrest.env - PostgREST environment variables", outputDir))

		userSQL := restGenerator.GenerateUserSetupSQL()
		filename = filepath.Join(outputDir, "setup_postgrest_users.sql")
		if err := writeFileWithSkip(filename, []byte(userSQL), skipExisting); err != nil {
			return nil, fmt.Errorf("failed to write setup_postgrest_users.sql: %w", err)
		}
		generatedFiles = append(generatedFiles, fmt.Sprintf("%s/setup_postgrest_users.sql - PostgREST user setup SQL", outputDir))

	case "all":
		fallthrough
	default:
		// Generate all configuration files
		pgConfig := pgGenerator.GenerateConfigFile()

		// Validate if requested
		if validate {
			fmt.Printf("Validating postgresql.conf...\n")
			postgres := pkg.NewPostgres(nil, "")
			if err := postgres.Validate([]byte(pgConfig)); err != nil {
				return nil, fmt.Errorf("postgresql.conf validation failed: %w", err)
			}
			fmt.Printf("✅ postgresql.conf validation passed\n")
		}

		filename := filepath.Join(outputDir, "postgresql.conf")
		if err := writeFileWithSkip(filename, []byte(pgConfig), skipExisting); err != nil {
			return nil, fmt.Errorf("failed to write postgresql.conf: %w", err)
		}
		generatedFiles = append(generatedFiles, fmt.Sprintf("%s/postgresql.conf - PostgreSQL configuration", outputDir))

		bouncerConfig := bouncerGenerator.GenerateConfigFile()

		// Validate if requested
		if validate {
			fmt.Printf("Validating pgbouncer.ini...\n")
			pgbouncer := pkg.NewPgBouncer(nil)
			if err := pgbouncer.Validate([]byte(bouncerConfig)); err != nil {
				return nil, fmt.Errorf("pgbouncer.ini validation failed: %w", err)
			}
			fmt.Printf("✅ pgbouncer.ini validation passed\n")
		}

		filename = filepath.Join(outputDir, "pgbouncer.ini")
		if err := writeFileWithSkip(filename, []byte(bouncerConfig), skipExisting); err != nil {
			return nil, fmt.Errorf("failed to write pgbouncer.ini: %w", err)
		}
		generatedFiles = append(generatedFiles, fmt.Sprintf("%s/pgbouncer.ini - PgBouncer connection pooler configuration", outputDir))

		restConfig, err := restGenerator.GenerateConfigFile()
		if err != nil {
			return nil, fmt.Errorf("failed to generate postgrest.conf: %w", err)
		}

		// Validate if requested
		if validate {
			fmt.Printf("Validating postgrest.conf...\n")
			postgrest := pkg.NewPostgREST(nil)
			if err := postgrest.Validate([]byte(restConfig)); err != nil {
				return nil, fmt.Errorf("postgrest.conf validation failed: %w", err)
			}
			fmt.Printf("✅ postgrest.conf validation passed\n")
		}

		filename = filepath.Join(outputDir, "postgrest.conf")
		if err := writeFileWithSkip(filename, []byte(restConfig), skipExisting); err != nil {
			return nil, fmt.Errorf("failed to write postgrest.conf: %w", err)
		}
		generatedFiles = append(generatedFiles, fmt.Sprintf("%s/postgrest.conf - PostgREST API configuration", outputDir))

		envConfig, err := restGenerator.GenerateEnvFile()
		if err != nil {
			return nil, fmt.Errorf("failed to generate postgrest.env: %w", err)
		}
		filename = filepath.Join(outputDir, "postgrest.env")
		if err := writeFileWithSkip(filename, []byte(envConfig), skipExisting); err != nil {
			return nil, fmt.Errorf("failed to write postgrest.env: %w", err)
		}
		generatedFiles = append(generatedFiles, fmt.Sprintf("%s/postgrest.env - PostgREST environment variables", outputDir))

		hbaConfig := hbaGenerator.GenerateConfigFile()
		filename = filepath.Join(outputDir, "pg_hba.conf")
		if err := writeFileWithSkip(filename, []byte(hbaConfig), skipExisting); err != nil {
			return nil, fmt.Errorf("failed to write pg_hba.conf: %w", err)
		}
		generatedFiles = append(generatedFiles, fmt.Sprintf("%s/pg_hba.conf - PostgreSQL authentication configuration", outputDir))

		userSQL := restGenerator.GenerateUserSetupSQL()
		filename = filepath.Join(outputDir, "setup_postgrest_users.sql")
		if err := writeFileWithSkip(filename, []byte(userSQL), skipExisting); err != nil {
			return nil, fmt.Errorf("failed to write setup_postgrest_users.sql: %w", err)
		}
		generatedFiles = append(generatedFiles, fmt.Sprintf("%s/setup_postgrest_users.sql - PostgREST user setup SQL", outputDir))

		// Generate PGAudit configuration if enabled
		if loadedConf != nil {
			pgauditGen := generators.NewPGAuditConfigGenerator(loadedConf.Pgaudit)
			if pgauditGen.IsEnabled() {
				pgauditConfig := pgauditGen.GenerateConfigFile()
				filename = filepath.Join(outputDir, "postgres.pgaudit.conf")
				if err := writeFileWithSkip(filename, []byte(pgauditConfig), skipExisting); err != nil {
					return nil, fmt.Errorf("failed to write postgres.pgaudit.conf: %w", err)
				}
				generatedFiles = append(generatedFiles, fmt.Sprintf("%s/postgres.pgaudit.conf - PGAudit extension configuration", outputDir))
			}
		}
	}

	return generatedFiles, nil
}

func writeFile(filename string, data []byte) error {
	return os.WriteFile(filename, data, 0644)
}

// writeFileWithSkip writes a file but can skip if it already exists
func writeFileWithSkip(filename string, data []byte, skipExisting bool) error {
	if skipExisting {
		if _, err := os.Stat(filename); err == nil {
			fmt.Printf("Skipping existing file: %s\n", filename)
			return nil
		}
	}
	return os.WriteFile(filename, data, 0644)
}

// detectPostgreSQLDataDir detects the PostgreSQL data directory
func detectPostgreSQLDataDir(overrideDir string) (string, error) {
	// If user provided an override, use it
	if overrideDir != "" {
		if _, err := os.Stat(overrideDir); os.IsNotExist(err) {
			return "", fmt.Errorf("specified PostgreSQL data directory does not exist: %s", overrideDir)
		}
		return overrideDir, nil
	}

	// Check PGDATA environment variable
	if pgdata := os.Getenv("PGDATA"); pgdata != "" {
		if _, err := os.Stat(pgdata); err == nil {
			return pgdata, nil
		}
	}

	// Check common locations based on OS
	osType := strings.ToLower(runtime.GOOS)

	var commonPaths []string
	switch osType {
	case "darwin":
		commonPaths = []string{
			"/usr/local/var/postgres",
			"/opt/homebrew/var/postgres",
			"/usr/local/var/postgresql",
			"/Library/PostgreSQL/*/data",
		}
	case "linux":
		commonPaths = []string{
			"/var/lib/postgresql/data",
			"/var/lib/pgsql/data",
			"/usr/local/pgsql/data",
			"/etc/postgresql",
			"/var/lib/postgresql/*/main",
		}
	case "windows":
		commonPaths = []string{
			"C:\\Program Files\\PostgreSQL\\*\\data",
			"C:\\PostgreSQL\\data",
		}
	default:
		commonPaths = []string{
			"/var/lib/postgresql/data",
			"/usr/local/pgsql/data",
		}
	}

	// Try each common path
	for _, path := range commonPaths {
		// Handle wildcard paths
		if strings.Contains(path, "*") {
			matches, err := filepath.Glob(path)
			if err == nil {
				for _, match := range matches {
					if stat, err := os.Stat(match); err == nil && stat.IsDir() {
						return match, nil
					}
				}
			}
		} else {
			if stat, err := os.Stat(path); err == nil && stat.IsDir() {
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("could not detect PostgreSQL data directory. Common locations checked: %v", commonPaths)
}

// getEnvString gets a string environment variable with a default value
func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets an integer environment variable with a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvFloat gets a float64 environment variable with a default value
func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

func handleValidateCommand() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Error: config type required\n")
		fmt.Fprintf(os.Stderr, "Usage: pgconfig validate <config-type> --file <path>\n")
		fmt.Fprintf(os.Stderr, "Valid config types: postgres, pgbouncer, postgrest\n")
		os.Exit(1)
	}

	configType := os.Args[2]

	// Validate config type
	validTypes := []string{"postgres", "pgbouncer", "postgrest"}
	isValid := false
	for _, validType := range validTypes {
		if configType == validType {
			isValid = true
			break
		}
	}

	if !isValid {
		fmt.Fprintf(os.Stderr, "Error: Unknown config type: %s\n", configType)
		fmt.Fprintf(os.Stderr, "Valid config types: postgres, pgbouncer, postgrest\n")
		os.Exit(1)
	}

	var (
		filePath = flag.String("file", "", "Configuration file path to validate (required)")
	)

	// Parse flags, skipping the validate command and config type
	flag.CommandLine.Parse(os.Args[3:])

	if *filePath == "" {
		fmt.Fprintf(os.Stderr, "Error: --file flag is required\n")
		fmt.Fprintf(os.Stderr, "Usage: pgconfig validate %s --file <path>\n", configType)
		os.Exit(1)
	}

	// Check if file exists
	if _, err := os.Stat(*filePath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: File does not exist: %s\n", *filePath)
		os.Exit(1)
	}

	// Read configuration file
	configContent, err := os.ReadFile(*filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to read config file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Validating %s configuration file: %s\n", configType, *filePath)

	// Validate based on config type
	var validationErr error
	switch configType {
	case "postgres":
		postgres := pkg.NewPostgres(nil, "")
		validationErr = postgres.Validate(configContent)

	case "pgbouncer":
		pgbouncer := pkg.NewPgBouncer(nil)
		validationErr = pgbouncer.Validate(configContent)

	case "postgrest":
		postgrest := pkg.NewPostgREST(nil)
		validationErr = postgrest.Validate(configContent)
	}

	if validationErr != nil {
		fmt.Fprintf(os.Stderr, "❌ Validation failed:\n")

		// Check if it's our ValidationError type for better formatting
		if valErr, ok := validationErr.(*pkg.ValidationError); ok {
			if valErr.Line > 0 {
				fmt.Fprintf(os.Stderr, "  Line %d: ", valErr.Line)
			}
			if valErr.Parameter != "" {
				fmt.Fprintf(os.Stderr, "Parameter '%s': ", valErr.Parameter)
			}
			fmt.Fprintf(os.Stderr, "%s\n", valErr.Message)
			if valErr.Raw != "" && valErr.Raw != valErr.Message {
				fmt.Fprintf(os.Stderr, "  Raw error: %s\n", valErr.Raw)
			}
		} else {
			fmt.Fprintf(os.Stderr, "  %s\n", validationErr.Error())
		}
		os.Exit(1)
	}

	fmt.Printf("✅ Configuration file is valid!\n")
}

func handleSupervisordCommand() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Error: supervisord subcommand required\n")
		fmt.Fprintf(os.Stderr, "Usage: pgconfig supervisord <subcommand> [args]\n")
		fmt.Fprintf(os.Stderr, "Subcommands:\n")
		fmt.Fprintf(os.Stderr, "  status [service]     Show status of all services or specific service\n")
		fmt.Fprintf(os.Stderr, "  start <service>      Start a service\n")
		fmt.Fprintf(os.Stderr, "  stop <service>       Stop a service\n")
		fmt.Fprintf(os.Stderr, "  restart <service>    Restart a service\n")
		fmt.Fprintf(os.Stderr, "  reload              Reload supervisord configuration\n")
		os.Exit(1)
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "status":
		handleSupervisordStatus()
	case "start":
		handleSupervisordStart()
	case "stop":
		handleSupervisordStop()
	case "restart":
		handleSupervisordRestart()
	case "reload":
		handleSupervisordReload()
	default:
		fmt.Fprintf(os.Stderr, "Error: Unknown supervisord subcommand: %s\n", subcommand)
		os.Exit(1)
	}
}

func handleSupervisordStatus() {
	var service string
	if len(os.Args) >= 4 {
		service = os.Args[3]
	}

	var cmd *exec.Cmd
	if service != "" {
		cmd = exec.Command("supervisorctl", "status", service)
	} else {
		cmd = exec.Command("supervisorctl", "status")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to get supervisord status: %v\n", err)
		fmt.Fprintf(os.Stderr, "Output: %s\n", string(output))
		os.Exit(1)
	}

	fmt.Print(string(output))
}

func handleSupervisordStart() {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Error: service name required\n")
		fmt.Fprintf(os.Stderr, "Usage: pgconfig supervisord start <service>\n")
		os.Exit(1)
	}

	service := os.Args[3]
	cmd := exec.Command("supervisorctl", "start", service)

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to start service %s: %v\n", service, err)
		fmt.Fprintf(os.Stderr, "Output: %s\n", string(output))
		os.Exit(1)
	}

	fmt.Printf("Service %s started successfully\n", service)
	fmt.Print(string(output))
}

func handleSupervisordStop() {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Error: service name required\n")
		fmt.Fprintf(os.Stderr, "Usage: pgconfig supervisord stop <service>\n")
		os.Exit(1)
	}

	service := os.Args[3]
	cmd := exec.Command("supervisorctl", "stop", service)

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to stop service %s: %v\n", service, err)
		fmt.Fprintf(os.Stderr, "Output: %s\n", string(output))
		os.Exit(1)
	}

	fmt.Printf("Service %s stopped successfully\n", service)
	fmt.Print(string(output))
}

func handleSupervisordRestart() {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Error: service name required\n")
		fmt.Fprintf(os.Stderr, "Usage: pgconfig supervisord restart <service>\n")
		os.Exit(1)
	}

	service := os.Args[3]
	cmd := exec.Command("supervisorctl", "restart", service)

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to restart service %s: %v\n", service, err)
		fmt.Fprintf(os.Stderr, "Output: %s\n", string(output))
		os.Exit(1)
	}

	fmt.Printf("Service %s restarted successfully\n", service)
	fmt.Print(string(output))
}

func handleSupervisordReload() {
	cmd := exec.Command("supervisorctl", "reload")

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to reload supervisord: %v\n", err)
		fmt.Fprintf(os.Stderr, "Output: %s\n", string(output))
		os.Exit(1)
	}

	fmt.Printf("Supervisord configuration reloaded successfully\n")
	fmt.Print(string(output))
}

// generateYAMLConfigFiles generates pgconfig.yaml and pgconfig.min.yaml files
func generateYAMLConfigFiles(outputDir string, loadedConf *pkg.Conf, skipExisting bool) ([]string, error) {
	var generatedFiles []string

	// Create a default config if none was loaded
	var conf *pkg.Conf
	if loadedConf != nil {
		conf = loadedConf
	} else {
		var err error
		conf, err = pkg.LoadConfig("") // Load with defaults only
		if err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
	}

	// Generate config generator
	generator := generators.NewConfigGenerator(conf)

	// Generate full YAML
	fullYAML, err := generator.GenerateFullYAML()
	if err != nil {
		return nil, fmt.Errorf("failed to generate full YAML: %w", err)
	}

	fullPath := filepath.Join(outputDir, "pgconfig.yaml")
	if err := writeFileWithSkip(fullPath, []byte(fullYAML), skipExisting); err != nil {
		return nil, fmt.Errorf("failed to write pgconfig.yaml: %w", err)
	}
	generatedFiles = append(generatedFiles, fmt.Sprintf("%s - Full configuration with all options and descriptions", fullPath))

	// Generate minimal YAML
	minimalYAML, err := generator.GenerateMinimalYAML()
	if err != nil {
		return nil, fmt.Errorf("failed to generate minimal YAML: %w", err)
	}

	minimalPath := filepath.Join(outputDir, "pgconfig.min.yaml")
	if err := writeFileWithSkip(minimalPath, []byte(minimalYAML), skipExisting); err != nil {
		return nil, fmt.Errorf("failed to write pgconfig.min.yaml: %w", err)
	}
	generatedFiles = append(generatedFiles, fmt.Sprintf("%s - Minimal configuration with non-default values only", minimalPath))

	return generatedFiles, nil
}

// handleInstallCommand handles the 'pgconfig install' command
func handleInstallCommand() {
	// Parse flags first
	flagSet := flag.NewFlagSet("install", flag.ExitOnError)
	configFile := flagSet.String("c", "", "Configuration file to read versions and enabled services from")
	version := flagSet.String("version", "", "Version to install (for single binary)")
	targetDir := flagSet.String("to", "", "Target directory for installation")

	// Parse flags from os.Args[2:] to handle both 'install list' and 'install <binary>'
	if len(os.Args) > 2 && !strings.HasPrefix(os.Args[2], "-") {
		// Handle subcommand (list or binary name)
		subcommand := os.Args[2]
		flagSet.Parse(os.Args[3:])

		if subcommand == "list" {
			handleInstallList()
			return
		}

		// Install specific binary
		handleInstallBinary(subcommand, *version, *targetDir)
	} else {
		// No subcommand, must use config file
		flagSet.Parse(os.Args[2:])
		if *configFile == "" {
			fmt.Println("Usage:")
			fmt.Println("  pgconfig install -c <config.yaml>    Install binaries based on config")
			fmt.Println("  pgconfig install <binary> [options]  Install specific binary")
			fmt.Println("  pgconfig install list                List available binaries")
			fmt.Println("")
			fmt.Println("Options:")
			fmt.Println("  --version <version>  Specify version to install")
			fmt.Println("  --to <directory>     Target directory for installation")
			os.Exit(1)
		}
		handleInstallFromConfig(*configFile, *targetDir)
	}
}

// handleInstallFromConfig installs binaries based on configuration file
func handleInstallFromConfig(configFile, targetDir string) {
	// Load the configuration
	config, err := pkg.LoadConfig(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	inst := installer.New()
	installed := []string{}
	skipped := []string{}

	// Install PostgREST if configured and enabled
	if config.Postgrest != nil && config.Postgrest.DbUri != "" {
		version := inst.GetDefaultVersion("postgrest")

		fmt.Printf("Installing PostgREST version %s...\n", version)
		if err := inst.InstallBinary("postgrest", version, targetDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error installing PostgREST: %v\n", err)
		} else {
			installed = append(installed, fmt.Sprintf("PostgREST (%s)", version))
		}
	} else {
		skipped = append(skipped, "PostgREST (not configured)")
	}

	// Install WAL-G if enabled
	if config.Walg != nil && config.Walg.Enabled {
		version := inst.GetDefaultVersion("wal-g")

		fmt.Printf("Installing WAL-G version %s...\n", version)
		if err := inst.InstallBinary("wal-g", version, targetDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error installing WAL-G: %v\n", err)
		} else {
			installed = append(installed, fmt.Sprintf("WAL-G (%s)", version))
		}
	} else {
		skipped = append(skipped, "WAL-G (not enabled)")
	}

	// Install PostgreSQL if postgres configuration exists
	if config.Postgres != nil {
		version := inst.GetDefaultVersion("postgres")
		fmt.Printf("Installing PostgreSQL version %s...\n", version)
		if err := inst.InstallBinary("postgres", version, targetDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error installing PostgreSQL: %v\n", err)
		} else {
			installed = append(installed, fmt.Sprintf("PostgreSQL (%s)", version))
		}
	} else {
		skipped = append(skipped, "PostgreSQL (no postgres configuration found)")
	}

	// Summary
	fmt.Println("\n=== Installation Summary ===")
	if len(installed) > 0 {
		fmt.Println("✅ Installed:")
		for _, s := range installed {
			fmt.Printf("   - %s\n", s)
		}
	}
	if len(skipped) > 0 {
		fmt.Println("⏭️  Skipped:")
		for _, s := range skipped {
			fmt.Printf("   - %s\n", s)
		}
	}
}

// handleInstallBinary installs a specific binary
func handleInstallBinary(binary, version, targetDir string) {
	inst := installer.New()

	if !inst.IsBinarySupported(binary) {
		fmt.Fprintf(os.Stderr, "Error: unsupported binary '%s'\n", binary)
		fmt.Printf("Supported binaries: %v\n", inst.ListSupportedBinaries())
		os.Exit(1)
	}

	// Show what we're installing
	fmt.Printf("Installing %s", binary)
	if version != "" {
		fmt.Printf(" version %s", version)
	} else {
		fmt.Printf(" (default: %s)", inst.GetDefaultVersion(binary))
	}
	if targetDir != "" {
		fmt.Printf(" to %s", targetDir)
	}
	fmt.Println("...")

	if err := inst.InstallBinary(binary, version, targetDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Successfully installed %s\n", binary)
}

// handleInstallList lists available binaries
func handleInstallList() {
	inst := installer.New()
	fmt.Println("Available binaries for installation:")
	for _, name := range inst.ListSupportedBinaries() {
		defaultVer := inst.GetDefaultVersion(name)
		installed := inst.CheckBinaryExists(name)
		status := ""
		if installed {
			if ver, err := inst.GetBinaryVersion(name); err == nil {
				status = fmt.Sprintf(" (installed: %s)", strings.TrimSpace(ver))
			} else {
				status = " (installed)"
			}
		}
		fmt.Printf("  - %s (default: %s)%s\n", name, defaultVer, status)
	}
}
