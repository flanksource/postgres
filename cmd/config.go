package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/flanksource/postgres/pkg"
	"github.com/flanksource/postgres/pkg/generators"
	"github.com/flanksource/postgres/pkg/installer"
	"github.com/flanksource/postgres/pkg/pgtune"
	"github.com/flanksource/postgres/pkg/server"
	"github.com/flanksource/postgres/pkg/sysinfo"
	"github.com/flanksource/postgres/pkg/utils"
	"github.com/spf13/cobra"
)

// createConfigCommands creates the config command group
func createConfigCommands() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "PostgreSQL configuration management",
		Long:  "Commands for generating, validating, and managing PostgreSQL configurations",
	}

	configCmd.AddCommand(
		createConfigGenerateCommand(),
		createConfigServerCommand(),
		createConfigValidateCommand(),
		createConfigSupervisordCommand(),
		createConfigInstallCommand(),
	)

	return configCmd
}

// createConfigGenerateCommand creates the config generate command
func createConfigGenerateCommand() *cobra.Command {
	generateCmd := &cobra.Command{
		Use:   "generate [config-type]",
		Short: "Generate configuration files",
		Long: `Generate PostgreSQL configuration files.

Config types:
  conf        Generate only postgresql.conf
  pgbouncer   Generate only pgbouncer.ini
  postgrest   Generate only postgrest.conf and postgrest.env
  (none)      Generate all configuration files (default)`,
		RunE: runConfigGenerate,
	}

	// Add flags
	generateCmd.Flags().Int("max-connections", 0, "Maximum number of connections (default: auto-detect)")
	generateCmd.Flags().String("db-type", "web", "Database type: web, oltp, dw, desktop, mixed")
	generateCmd.Flags().String("output-dir", "./config", "Output directory for config files")
	generateCmd.Flags().Float64("memory", 0, "Override detected memory in GB")
	generateCmd.Flags().Int("cpus", 0, "Override detected CPU count")
	generateCmd.Flags().Float64("pg-version", 0, "Override PostgreSQL version")
	generateCmd.Flags().Bool("save", false, "Save configs to system PostgreSQL directories")
	generateCmd.Flags().Bool("skip-existing", false, "Skip files that already exist")
	generateCmd.Flags().String("pg-data-dir", "", "Override PostgreSQL data directory")
	generateCmd.Flags().Bool("validate", false, "Validate generated configs before saving")

	return generateCmd
}

// createConfigServerCommand creates the config server command
func createConfigServerCommand() *cobra.Command {
	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "Start health check server",
		Long:  "Start a health check server that serves PostgreSQL configuration files",
		RunE:  runConfigServer,
	}

	serverCmd.Flags().Int("port", 8080, "Server port")
	serverCmd.Flags().String("config-dir", "./config", "Directory to serve configs from")
	serverCmd.Flags().Int("max-connections", 0, "Maximum number of connections")
	serverCmd.Flags().String("db-type", "web", "Database type: web, oltp, dw, desktop, mixed")

	return serverCmd
}

// createConfigValidateCommand creates the config validate command
func createConfigValidateCommand() *cobra.Command {
	validateCmd := &cobra.Command{
		Use:   "validate <config-type>",
		Short: "Validate configuration files",
		Long: `Validate PostgreSQL configuration files.

Config types:
  postgres    Validate postgresql.conf file
  pgbouncer   Validate pgbouncer.ini file
  postgrest   Validate postgrest.conf file`,
		Args: cobra.ExactArgs(1),
		RunE: runConfigValidate,
	}

	validateCmd.Flags().String("file", "", "Configuration file path to validate (required)")
	validateCmd.MarkFlagRequired("file")

	return validateCmd
}

// createConfigSupervisordCommand creates the config supervisord command
func createConfigSupervisordCommand() *cobra.Command {
	supervisordCmd := &cobra.Command{
		Use:   "supervisord <subcommand>",
		Short: "Manage supervisord services",
		Long: `Manage supervisord services.

Subcommands:
  status [service]     Show status of all services or specific service
  start <service>      Start a service
  stop <service>       Stop a service
  restart <service>    Restart a service
  reload              Reload supervisord configuration`,
		Args: cobra.MinimumNArgs(1),
		RunE: runConfigSupervisord,
	}

	return supervisordCmd
}

// createConfigInstallCommand creates the config install command
func createConfigInstallCommand() *cobra.Command {
	installCmd := &cobra.Command{
		Use:   "install [binary]",
		Short: "Install binary tools",
		Long: `Install binary tools (postgres, postgrest, wal-g).

Examples:
  config install                          Install binaries based on config file
  config install postgres                 Install PostgreSQL with default version
  config install postgres --version 15.5.0  Install specific PostgreSQL version
  config install list                     List available binaries`,
		RunE: runConfigInstall,
	}

	installCmd.Flags().String("version", "", "Specify version to install")
	installCmd.Flags().String("to", "", "Target directory for installation")

	return installCmd
}

// runConfigGenerate handles the config generate command
func runConfigGenerate(cmd *cobra.Command, args []string) error {
	configType := "all"
	if len(args) > 0 {
		configType = args[0]
	}

	// Get flag values
	maxConnections, _ := cmd.Flags().GetInt("max-connections")
	dbType, _ := cmd.Flags().GetString("db-type")
	outputDir, _ := cmd.Flags().GetString("output-dir")
	memoryGB, _ := cmd.Flags().GetFloat64("memory")
	cpus, _ := cmd.Flags().GetInt("cpus")
	pgVersion, _ := cmd.Flags().GetFloat64("pg-version")
	saveToSystem, _ := cmd.Flags().GetBool("save")
	skipExisting, _ := cmd.Flags().GetBool("skip-existing")
	pgDataDir, _ := cmd.Flags().GetString("pg-data-dir")
	validate, _ := cmd.Flags().GetBool("validate")

	// Load configuration from file if provided
	var loadedConf *pkg.Conf
	if configFile != "" {
		var err error
		loadedConf, err = pkg.LoadConfig(configFile)
		if err != nil {
			return fmt.Errorf("error loading config file: %w", err)
		}
		if verbose {
			fmt.Printf("Loaded configuration from: %s\n", configFile)
		}
	}

	// Handle --save flag to determine system PostgreSQL directory
	if saveToSystem {
		systemDir, err := detectPostgreSQLDataDir(pgDataDir)
		if err != nil {
			return fmt.Errorf("could not detect PostgreSQL data directory: %w. Use --pg-data-dir to specify manually or remove --save flag", err)
		}
		outputDir = systemDir
		if verbose {
			fmt.Printf("Saving to PostgreSQL system directory: %s\n", systemDir)
		}
	}

	if verbose {
		fmt.Printf("Generating PostgreSQL configuration files...\n")
	}

	// Detect system information
	sysInfo, err := sysinfo.DetectSystemInfo()
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: Could not detect system info: %v\n", err)
		}
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
	if memoryGB > 0 {
		if memoryGB < 0.1 {
			return fmt.Errorf("memory must be at least 0.1 GB, got %.2f", memoryGB)
		}
		oldMemoryGB := float64(sysInfo.TotalMemoryBytes) / (1024 * 1024 * 1024)
		sysInfo.TotalMemoryBytes = uint64(memoryGB * 1024 * 1024 * 1024)
		if verbose {
			fmt.Printf("Memory override: %.1f GB (was %.1f GB)\n", memoryGB, oldMemoryGB)
		}
	}
	if cpus > 0 {
		oldCPUs := sysInfo.CPUCount
		sysInfo.CPUCount = cpus
		if verbose {
			fmt.Printf("CPU override: %d (was %d)\n", cpus, oldCPUs)
		}
	}
	if pgVersion > 0 {
		oldVersion := sysInfo.PostgreSQLVersion
		sysInfo.PostgreSQLVersion = pgVersion
		if verbose {
			fmt.Printf("PostgreSQL version override: %.1f (was %.1f)\n", pgVersion, oldVersion)
		}
	}

	// Parse database type
	dbTypeEnum, err := parseDBType(dbType)
	if err != nil {
		return err
	}

	// Calculate max connections if not provided
	if maxConnections == 0 {
		maxConnections = pgtune.GetRecommendedMaxConnections(dbTypeEnum)
	}

	if verbose {
		fmt.Printf("System Information:\n")
		fmt.Printf("  Memory: %.1f GB\n", sysInfo.TotalMemoryGB())
		fmt.Printf("  CPUs: %d\n", sysInfo.CPUCount)
		fmt.Printf("  OS: %s\n", sysInfo.OSType)
		fmt.Printf("  PostgreSQL Version: %.1f\n", sysInfo.PostgreSQLVersion)
		fmt.Printf("  Disk Type: %s\n", sysInfo.DiskType)
		fmt.Printf("\nConfiguration:\n")
		fmt.Printf("  Max Connections: %d\n", maxConnections)
		fmt.Printf("  Database Type: %s\n", dbType)
		fmt.Printf("  Output Directory: %s\n", outputDir)
		fmt.Printf("\n")
	}

	// Generate tuned parameters
	tuningConfig := &pgtune.TuningConfig{
		SystemInfo:     sysInfo,
		MaxConnections: maxConnections,
		DBType:         dbTypeEnum,
	}

	params, err := pgtune.CalculateOptimalConfig(tuningConfig)
	if err != nil {
		return fmt.Errorf("error calculating optimal config: %w", err)
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("error creating output directory: %w", err)
	}

	// Save actual files based on config type
	generatedFiles, err := saveSpecificConfigFiles(configType, outputDir, sysInfo, params, maxConnections, dbTypeEnum, skipExisting, validate, loadedConf)
	if err != nil {
		return fmt.Errorf("error writing config files: %w", err)
	}

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

	return nil
}

// runConfigServer handles the config server command
func runConfigServer(cmd *cobra.Command, args []string) error {
	port, _ := cmd.Flags().GetInt("port")
	configDir, _ := cmd.Flags().GetString("config-dir")
	maxConnections, _ := cmd.Flags().GetInt("max-connections")
	dbType, _ := cmd.Flags().GetString("db-type")

	if verbose {
		fmt.Printf("Starting PostgreSQL configuration health server...\n")
	}

	// Parse database type
	dbTypeEnum, err := parseDBType(dbType)
	if err != nil {
		return err
	}

	// Calculate max connections if not provided
	if maxConnections == 0 {
		maxConnections = pgtune.GetRecommendedMaxConnections(dbTypeEnum)
	}

	// Create and configure health server
	healthServer := server.NewHealthServer(port, configDir)

	if err := healthServer.ConfigFromFile(maxConnections, dbTypeEnum); err != nil {
		return fmt.Errorf("error configuring server: %w", err)
	}

	if verbose {
		fmt.Printf("Configuration:\n")
		fmt.Printf("  Port: %d\n", port)
		fmt.Printf("  Config Directory: %s\n", configDir)
		fmt.Printf("  Max Connections: %d\n", maxConnections)
		fmt.Printf("  Database Type: %s\n", dbType)
		fmt.Printf("\nEndpoints:\n")
		fmt.Printf("  http://localhost:%d/           - API documentation\n", port)
		fmt.Printf("  http://localhost:%d/live       - Liveness check\n", port)
		fmt.Printf("  http://localhost:%d/ready      - Readiness check\n", port)
		fmt.Printf("  http://localhost:%d/info       - System information\n", port)
		fmt.Printf("  http://localhost:%d/config     - Configuration summary\n", port)
		fmt.Printf("  http://localhost:%d/health/status           - Detailed health status\n", port)
		fmt.Printf("  http://localhost:%d/config/postgresql.conf  - PostgreSQL config\n", port)
		fmt.Printf("  http://localhost:%d/config/pgbouncer.ini    - PgBouncer config\n", port)
		fmt.Printf("  http://localhost:%d/config/postgrest.conf   - PostgREST config\n", port)
		fmt.Printf("  http://localhost:%d/config/pg_hba.conf      - Authentication config\n", port)
		fmt.Printf("\n")
	}

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
		return fmt.Errorf("error during shutdown: %w", err)
	}

	fmt.Printf("Server stopped successfully\n")
	return nil
}

// runConfigValidate handles the config validate command
func runConfigValidate(cmd *cobra.Command, args []string) error {
	configType := args[0]
	filePath, _ := cmd.Flags().GetString("file")

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
		return fmt.Errorf("unknown config type: %s. Valid types: postgres, pgbouncer, postgrest", configType)
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", filePath)
	}

	// Read configuration file
	configContent, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if verbose {
		fmt.Printf("Validating %s configuration file: %s\n", configType, filePath)
	}

	// Validate based on config type
	var validationErr error
	switch configType {
	case "postgres":
		postgres := getPostgresInstance()
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
		return validationErr
	}

	fmt.Printf("✅ Configuration file is valid!\n")
	return nil
}

// runConfigSupervisord handles the config supervisord command
func runConfigSupervisord(cmd *cobra.Command, args []string) error {
	subcommand := args[0]

	switch subcommand {
	case "status":
		return handleSupervisordStatus(args[1:])
	case "start":
		if len(args) < 2 {
			return fmt.Errorf("service name required for start command")
		}
		return handleSupervisordStart(args[1])
	case "stop":
		if len(args) < 2 {
			return fmt.Errorf("service name required for stop command")
		}
		return handleSupervisordStop(args[1])
	case "restart":
		if len(args) < 2 {
			return fmt.Errorf("service name required for restart command")
		}
		return handleSupervisordRestart(args[1])
	case "reload":
		return handleSupervisordReload()
	default:
		return fmt.Errorf("unknown supervisord subcommand: %s", subcommand)
	}
}

// runConfigInstall handles the config install command
func runConfigInstall(cmd *cobra.Command, args []string) error {
	version, _ := cmd.Flags().GetString("version")
	targetDir, _ := cmd.Flags().GetString("to")

	if len(args) == 0 {
		// Install from config file
		if configFile == "" {
			return fmt.Errorf("either specify a binary name or use --config flag")
		}
		return handleInstallFromConfig(configFile, targetDir)
	}

	binary := args[0]
	if binary == "list" {
		return handleInstallList()
	}

	return handleInstallBinary(binary, version, targetDir)
}

// Helper functions (implementations from original pgconfig main.go)

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

func detectPostgreSQLDataDir(overrideDir string) (string, error) {
	// If user provided an override, use it
	if overrideDir != "" {
		if _, err := os.Stat(overrideDir); os.IsNotExist(err) {
			return "", fmt.Errorf("specified PostgreSQL data directory does not exist: %s", overrideDir)
		}
		return overrideDir, nil
	}

	// Use the auto-detection from utils
	dataDir, err := utils.DetectDataDir()
	if err != nil {
		return "", err
	}

	return dataDir, nil
}

func saveSpecificConfigFiles(configType, outputDir string, sysInfo *sysinfo.SystemInfo, params *pgtune.TunedParameters, maxConnections int, dbType sysinfo.DBType, skipExisting bool, validate bool, loadedConf *pkg.Conf) ([]string, error) {
	var generatedFiles []string

	// Create generators
	pgGenerator := generators.NewPostgreSQLConfigGenerator(sysInfo, params)
	bouncerGenerator := generators.NewPgBouncerConfigGenerator(sysInfo, params)
	restGenerator := generators.NewPostgRESTConfigGenerator(sysInfo, params)
	_ = generators.NewPgHBAConfigGenerator(sysInfo) // hbaGenerator not used in this simplified version

	// Configure PGAudit if loaded configuration is available
	if loadedConf != nil {
		pgGenerator.SetPGAuditConf(loadedConf.Pgaudit)
	}

	switch configType {
	case "conf":
		// Generate only PostgreSQL config
		pgConfig := pgGenerator.GenerateConfigFile()

		// Validate if requested
		if validate {
			if verbose {
				fmt.Printf("Validating postgresql.conf...\n")
			}
			postgres := getPostgresInstance()
			if err := postgres.Validate([]byte(pgConfig)); err != nil {
				return nil, fmt.Errorf("postgresql.conf validation failed: %w", err)
			}
			if verbose {
				fmt.Printf("✅ postgresql.conf validation passed\n")
			}
		}

		filename := filepath.Join(outputDir, "postgresql.conf")
		if err := writeFileWithSkip(filename, []byte(pgConfig), skipExisting); err != nil {
			return nil, fmt.Errorf("failed to write postgresql.conf: %w", err)
		}
		generatedFiles = append(generatedFiles, fmt.Sprintf("%s/postgresql.conf - PostgreSQL configuration", outputDir))

	case "pgbouncer":
		// Generate only PgBouncer config
		bouncerConfig := bouncerGenerator.GenerateConfigFile()

		// Validate if requested
		if validate {
			if verbose {
				fmt.Printf("Validating pgbouncer.ini...\n")
			}
			pgbouncer := pkg.NewPgBouncer(nil)
			if err := pgbouncer.Validate([]byte(bouncerConfig)); err != nil {
				return nil, fmt.Errorf("pgbouncer.ini validation failed: %w", err)
			}
			if verbose {
				fmt.Printf("✅ pgbouncer.ini validation passed\n")
			}
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
			if verbose {
				fmt.Printf("Validating postgrest.conf...\n")
			}
			postgrest := pkg.NewPostgREST(nil)
			if err := postgrest.Validate([]byte(restConfig)); err != nil {
				return nil, fmt.Errorf("postgrest.conf validation failed: %w", err)
			}
			if verbose {
				fmt.Printf("✅ postgrest.conf validation passed\n")
			}
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

	case "all":
		fallthrough
	default:
		// Generate all configuration files (similar implementation to original)
		// This would be a longer implementation including all config types
		return nil, fmt.Errorf("config type '%s' not fully implemented yet", configType)
	}

	return generatedFiles, nil
}

func writeFileWithSkip(filename string, data []byte, skipExisting bool) error {
	if skipExisting {
		if _, err := os.Stat(filename); err == nil {
			if verbose {
				fmt.Printf("Skipping existing file: %s\n", filename)
			}
			return nil
		}
	}
	return os.WriteFile(filename, data, 0644)
}

// Supervisord helper functions
func handleSupervisordStatus(args []string) error {
	var service string
	if len(args) > 0 {
		service = args[0]
	}

	var cmd *exec.Cmd
	if service != "" {
		cmd = exec.Command("supervisorctl", "status", service)
	} else {
		cmd = exec.Command("supervisorctl", "status")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get supervisord status: %w. Output: %s", err, string(output))
	}

	fmt.Print(string(output))
	return nil
}

func handleSupervisordStart(service string) error {
	cmd := exec.Command("supervisorctl", "start", service)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start service %s: %w. Output: %s", service, err, string(output))
	}

	fmt.Printf("Service %s started successfully\n", service)
	fmt.Print(string(output))
	return nil
}

func handleSupervisordStop(service string) error {
	cmd := exec.Command("supervisorctl", "stop", service)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stop service %s: %w. Output: %s", service, err, string(output))
	}

	fmt.Printf("Service %s stopped successfully\n", service)
	fmt.Print(string(output))
	return nil
}

func handleSupervisordRestart(service string) error {
	cmd := exec.Command("supervisorctl", "restart", service)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restart service %s: %w. Output: %s", service, err, string(output))
	}

	fmt.Printf("Service %s restarted successfully\n", service)
	fmt.Print(string(output))
	return nil
}

func handleSupervisordReload() error {
	cmd := exec.Command("supervisorctl", "reload")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to reload supervisord: %w. Output: %s", err, string(output))
	}

	fmt.Printf("Supervisord configuration reloaded successfully\n")
	fmt.Print(string(output))
	return nil
}

// Install helper functions
func handleInstallFromConfig(configFile, targetDir string) error {
	// Load the configuration
	config, err := pkg.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	inst := installer.New()
	installed := []string{}
	skipped := []string{}

	// Install PostgREST if configured and enabled
	if config.Postgrest != nil && config.Postgrest.DbUri != nil && *config.Postgrest.DbUri != "" {
		version := inst.GetDefaultVersion("postgrest")

		if verbose {
			fmt.Printf("Installing PostgREST version %s...\n", version)
		}
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

		if verbose {
			fmt.Printf("Installing WAL-G version %s...\n", version)
		}
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
		if verbose {
			fmt.Printf("Installing PostgreSQL version %s...\n", version)
		}
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

	return nil
}

func handleInstallBinary(binary, version, targetDir string) error {
	inst := installer.New()

	if !inst.IsBinarySupported(binary) {
		return fmt.Errorf("unsupported binary '%s'. Supported binaries: %v", binary, inst.ListSupportedBinaries())
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
		return err
	}

	fmt.Printf("✅ Successfully installed %s\n", binary)
	return nil
}

func handleInstallList() error {
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
	return nil
}