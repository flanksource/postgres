package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/flanksource/clicky"
	"github.com/flanksource/postgres/pkg/generators"
	"github.com/flanksource/postgres/pkg/pgtune"
	"github.com/flanksource/postgres/pkg/sysinfo"
	"github.com/flanksource/postgres/pkg/utils"
)

const version = "1.0.0"

var (
	// Global flags
	dataDir    string
	binDir     string
	verbose    bool
	configFile string

	// Auto-detected directories
	detectedDirs *utils.PostgreSQLDirs
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "postgres-cli",
		Short: "PostgreSQL Management CLI",
		Long: `A comprehensive CLI tool for managing PostgreSQL servers, generating configurations, and working with schemas.
This unified tool combines PostgreSQL server management, configuration generation, and schema operations.`,
		Version: version,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			clicky.Flags.UseFlags()
			initializeDirectories()
		},
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", "", "PostgreSQL data directory (auto-detected if not specified)")
	rootCmd.PersistentFlags().StringVar(&binDir, "bin-dir", "", "PostgreSQL binary directory (auto-detected if not specified)")
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Configuration file path")

	clicky.BindAllFlags(rootCmd.PersistentFlags())

	// Add command groups
	rootCmd.AddCommand(
		createServerCommands(),
		createPgTuneCommand(),
		createAutoStartCommand(),
		createVersionCommand(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("Error: %+v\n", err)
		os.Exit(1)
	}
}

// initializeDirectories detects PostgreSQL directories and applies overrides
func initializeDirectories() {
	var err error

	// Try to auto-detect directories
	detectedDirs, err = utils.DetectPostgreSQLDirs()
	if err != nil && verbose {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}

	// Apply overrides from flags or use detected values
	if binDir == "" && detectedDirs != nil {
		binDir = detectedDirs.BinDir
	}
	if dataDir == "" && detectedDirs != nil {
		dataDir = detectedDirs.DataDir
	}

	// Show detected/configured directories in verbose mode
	if verbose {
		fmt.Fprintf(os.Stderr, "Using binary directory: %s\n", binDir)
		fmt.Fprintf(os.Stderr, "Using data directory: %s\n", dataDir)
	}
}

// createPgTuneCommand creates the pg-tune command
func createPgTuneCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pg-tune",
		Short: "Generate optimized PostgreSQL configuration",
		Long: `Generate tuned PostgreSQL configuration parameters based on system resources.

This command analyzes your system (memory, CPUs, disk type) and generates
optimal PostgreSQL configuration parameters for your workload type.

Examples:
  pgconfig pg-tune                                Generate config with defaults (web type)
  pgconfig pg-tune --type oltp                    Generate config for OLTP workload
  pgconfig pg-tune --type web --max-connections 200  Tune for web with 200 connections
  pgconfig pg-tune --memory 16 --cpus 8           Override detected resources
  pgconfig pg-tune --output full                  Generate full postgresql.conf`,
		RunE: runPgTune,
	}

	cmd.Flags().String("type", "web", "Database type: web, oltp, dw, desktop, mixed")
	cmd.Flags().Int("max-connections", 0, "Maximum connections (0 = auto-calculate based on type)")
	cmd.Flags().String("output", "snippet", "Output format: snippet, full, json")
	cmd.Flags().Float64("memory", 0, "Override detected memory in GB")
	cmd.Flags().Int("cpus", 0, "Override detected CPU count")
	cmd.Flags().Bool("save", false, "Save to data-dir/postgresql.conf")
	cmd.Flags().Bool("update", false, "Update data-dir/postgresql.auto.conf (merges with existing params)")

	return cmd
}

// runPgTune handles the pg-tune command execution
func runPgTune(cmd *cobra.Command, args []string) error {
	dbTypeStr, _ := cmd.Flags().GetString("type")
	maxConnections, _ := cmd.Flags().GetInt("max-connections")
	outputFormat, _ := cmd.Flags().GetString("output")
	memoryGB, _ := cmd.Flags().GetFloat64("memory")
	cpus, _ := cmd.Flags().GetInt("cpus")
	save, _ := cmd.Flags().GetBool("save")
	update, _ := cmd.Flags().GetBool("update")

	// Validate mutually exclusive flags
	if save && update {
		return fmt.Errorf("--save and --update flags are mutually exclusive")
	}

	// Parse database type
	var dbType sysinfo.DBType
	switch dbTypeStr {
	case "web":
		dbType = sysinfo.DBTypeWeb
	case "oltp":
		dbType = sysinfo.DBTypeOLTP
	case "dw":
		dbType = sysinfo.DBTypeDW
	case "desktop":
		dbType = sysinfo.DBTypeDesktop
	case "mixed":
		dbType = sysinfo.DBTypeMixed
	default:
		return fmt.Errorf("invalid database type: %s (valid: web, oltp, dw, desktop, mixed)", dbTypeStr)
	}

	// Detect system information
	sysInfo, err := sysinfo.DetectSystemInfo()
	if err != nil {
		return fmt.Errorf("failed to detect system info: %w", err)
	}

	// Apply overrides
	if memoryGB > 0 {
		sysInfo.TotalMemoryBytes = uint64(memoryGB * float64(pgtune.GB))
	}
	if cpus > 0 {
		sysInfo.CPUCount = cpus
	}

	// Calculate max connections if not specified
	if maxConnections == 0 {
		maxConnections = pgtune.GetRecommendedMaxConnections(dbType)
	}

	// Create tuning config
	tuningConfig := &pgtune.TuningConfig{
		SystemInfo:     sysInfo,
		MaxConnections: maxConnections,
		DBType:         dbType,
	}

	// Calculate optimal configuration
	params, err := pgtune.CalculateOptimalConfig(tuningConfig)
	if err != nil {
		return fmt.Errorf("failed to calculate optimal config: %w", err)
	}

	// Generate output based on format
	var output string
	switch outputFormat {
	case "snippet":
		output = generateConfigSnippet(sysInfo, params, dbType)
	case "full":
		generator := generators.NewPostgreSQLConfigGenerator(sysInfo, params)
		output = generator.GenerateConfigFile()
	case "json":
		// Generate JSON representation of tuned parameters
		output, err = generateJSONConfig(params)
		if err != nil {
			return fmt.Errorf("failed to generate JSON: %w", err)
		}
	default:
		return fmt.Errorf("invalid output format: %s (valid: snippet, full, json)", outputFormat)
	}

	// Update postgres.auto.conf if requested
	if update {
		dataDir := getDataDir()
		if dataDir == "" {
			return fmt.Errorf("data directory not detected, use --data-dir flag")
		}

		autoConfPath := filepath.Join(dataDir, "postgresql.auto.conf")

		// Parse existing auto.conf
		autoConf, err := pgtune.ParseAutoConf(autoConfPath)
		if err != nil {
			return fmt.Errorf("failed to parse postgresql.auto.conf: %w", err)
		}

		// Merge with tuned parameters
		autoConf.MergeWithTunedParams(params)

		// Write back to file
		if err := autoConf.WriteToFile(autoConfPath); err != nil {
			return fmt.Errorf("failed to write postgresql.auto.conf: %w", err)
		}

		fmt.Printf("✅ Configuration updated in: %s\n", autoConfPath)
		fmt.Println("ℹ️  Only pg_tune managed parameters were updated, other settings preserved")
		return nil
	}

	// Save to file if requested
	if save {
		dataDir := getDataDir()
		if dataDir == "" {
			return fmt.Errorf("data directory not detected, use --data-dir flag")
		}

		configPath := filepath.Join(dataDir, "postgresql.conf")
		if err := os.WriteFile(configPath, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write postgresql.conf: %w", err)
		}

		fmt.Printf("✅ Configuration saved to: %s\n", configPath)
		return nil
	}

	// Print to stdout
	fmt.Print(output)
	return nil
}

// generateConfigSnippet generates a snippet of key tuning parameters
func generateConfigSnippet(sysInfo *sysinfo.SystemInfo, params *pgtune.TunedParameters, dbType sysinfo.DBType) string {
	var snippet strings.Builder

	snippet.WriteString("# Generated by pgconfig pg-tune\n")
	snippet.WriteString(fmt.Sprintf("# System: %.1f GB RAM, %d CPUs\n",
		float64(sysInfo.TotalMemoryBytes)/float64(pgtune.GB), sysInfo.CPUCount))
	snippet.WriteString(fmt.Sprintf("# Type: %s, Max Connections: %d\n\n", dbType, params.MaxConnections))

	// Memory settings
	snippet.WriteString("# Memory Configuration\n")
	snippet.WriteString(fmt.Sprintf("shared_buffers = %dMB\n", params.SharedBuffers/1024))
	snippet.WriteString(fmt.Sprintf("effective_cache_size = %dMB\n", params.EffectiveCacheSize/1024))
	snippet.WriteString(fmt.Sprintf("maintenance_work_mem = %dMB\n", params.MaintenanceWorkMem/1024))
	snippet.WriteString(fmt.Sprintf("work_mem = %dMB\n\n", params.WorkMem/1024))

	// WAL settings
	snippet.WriteString("# WAL Configuration\n")
	snippet.WriteString(fmt.Sprintf("wal_buffers = %dMB\n", params.WalBuffers/1024))
	snippet.WriteString(fmt.Sprintf("min_wal_size = %dMB\n", params.MinWalSize/1024))
	snippet.WriteString(fmt.Sprintf("max_wal_size = %dMB\n", params.MaxWalSize/1024))
	snippet.WriteString(fmt.Sprintf("checkpoint_completion_target = %.2f\n\n", params.CheckpointCompletionTarget))

	// Performance settings
	snippet.WriteString("# Query Planner Configuration\n")
	snippet.WriteString(fmt.Sprintf("random_page_cost = %.1f\n", params.RandomPageCost))
	if params.EffectiveIoConcurrency != nil {
		snippet.WriteString(fmt.Sprintf("effective_io_concurrency = %d\n", *params.EffectiveIoConcurrency))
	}
	snippet.WriteString(fmt.Sprintf("default_statistics_target = %d\n\n", params.DefaultStatisticsTarget))

	// Parallel processing
	snippet.WriteString("# Parallel Processing\n")
	snippet.WriteString(fmt.Sprintf("max_worker_processes = %d\n", params.MaxWorkerProcesses))
	snippet.WriteString(fmt.Sprintf("max_parallel_workers = %d\n", params.MaxParallelWorkers))
	snippet.WriteString(fmt.Sprintf("max_parallel_workers_per_gather = %d\n", params.MaxParallelWorkersPerGather))
	if params.MaxParallelMaintenanceWorkers != nil {
		snippet.WriteString(fmt.Sprintf("max_parallel_maintenance_workers = %d\n", *params.MaxParallelMaintenanceWorkers))
	}

	// Add warnings if any
	if len(params.Warnings) > 0 {
		snippet.WriteString("\n# Warnings:\n")
		for _, warning := range params.Warnings {
			snippet.WriteString(fmt.Sprintf("# %s\n", warning))
		}
	}

	return snippet.String()
}

// generateJSONConfig generates JSON representation of tuned parameters
func generateJSONConfig(params *pgtune.TunedParameters) (string, error) {
	data, err := json.MarshalIndent(params, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// createAutoStartCommand creates the auto-start command
func createAutoStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "auto-start",
		SilenceUsage: true,
		Short:        "Automatically start PostgreSQL with optional pre-start tasks",
		Long: `Start PostgreSQL server with automatic setup tasks.

This command can automatically:
- Initialize database if data directory doesn't exist
- Upgrade PostgreSQL to a target version if needed
- Optimize configuration using pg_tune
- Reset the superuser password

Examples:
  pgconfig auto-start                           Start PostgreSQL normally
  pgconfig auto-start --auto-init               Initialize and start if needed
  pgconfig auto-start --pg-tune                 Optimize config before starting
  pgconfig auto-start --auto-upgrade            Upgrade if needed, then start
  pgconfig auto-start --auto-reset-password     Reset password, then start
  pgconfig auto-start --auto-init --pg-tune     Initialize, optimize, then start`,
		RunE: runAutoStart,
	}

	cmd.Flags().Bool("auto-init", false, "Automatically initialize database if data directory doesn't exist")
	cmd.Flags().Bool("pg-tune", false, "Run pg_tune to optimize postgresql.conf before starting")
	cmd.Flags().Bool("auto-upgrade", false, "Automatically upgrade PostgreSQL if version mismatch detected")
	cmd.Flags().Bool("auto-reset-password", false, "Reset postgres superuser password on start")
	cmd.Flags().Int("upgrade-to", 0, "Target PostgreSQL version for upgrade (default: auto-detect latest)")
	cmd.Flags().String("new-password", "", "New password for auto-reset (prompted if not provided)")

	return cmd
}

// runAutoStart handles the auto-start command execution
func runAutoStart(cmd *cobra.Command, args []string) error {
	autoInit, _ := cmd.Flags().GetBool("auto-init")
	pgTune, _ := cmd.Flags().GetBool("pg-tune")
	autoUpgrade, _ := cmd.Flags().GetBool("auto-upgrade")
	autoResetPassword, _ := cmd.Flags().GetBool("auto-reset-password")
	upgradeTo, _ := cmd.Flags().GetInt("upgrade-to")
	newPassword, _ := cmd.Flags().GetString("new-password")

	// Get PostgreSQL instance
	postgres := getPostgresInstance()

	// Check if already running
	if postgres.IsRunning() {
		fmt.Println("PostgreSQL is already running")
		return nil
	}

	// Step 1: Auto-init if data directory doesn't exist
	if !postgres.Exists() {
		if autoInit {
			fmt.Println("🔧 Initializing PostgreSQL database...")
			if err := postgres.InitDB(); err != nil {
				return fmt.Errorf("failed to initialize database: %w", err)
			}
			fmt.Println("✅ PostgreSQL database initialized successfully")
		} else {
			return fmt.Errorf("PostgreSQL data directory does not exist. Use --auto-init to initialize automatically")
		}
	}

	// Step 2: Auto-upgrade if requested
	if autoUpgrade {
		currentVersion, err := postgres.DetectVersion()
		if err != nil {
			return fmt.Errorf("failed to detect current PostgreSQL version: %w", err)
		}

		targetVersion := upgradeTo
		if targetVersion == 0 {
			// Auto-detect latest available version (default to 17)
			targetVersion = 17
		}

		if currentVersion < targetVersion {
			fmt.Printf("🚀 Upgrading PostgreSQL from version %d to %d...\n", currentVersion, targetVersion)
			if err := postgres.Upgrade(targetVersion); err != nil {
				return fmt.Errorf("failed to upgrade PostgreSQL: %w", err)
			}
			fmt.Println("✅ PostgreSQL upgrade completed successfully")
		} else {
			fmt.Printf("✅ PostgreSQL is already at version %d (target: %d)\n", currentVersion, targetVersion)
		}
	}

	// Step 3: Run pg_tune if requested
	if pgTune {
		fmt.Println("🔧 Running pg_tune to optimize configuration...")

		dataDir := getDataDir()
		if dataDir == "" {
			return fmt.Errorf("data directory not detected")
		}

		err := pgtune.OptimizeAndSave(pgtune.OptimizeOptions{
			DataDir: dataDir,
			DBType:  sysinfo.DBTypeWeb,
		})
		if err != nil {
			return fmt.Errorf("failed to run pg_tune: %w", err)
		}

		fmt.Println("✅ Configuration optimized successfully")
	}

	// Step 4: Reset password if requested
	if autoResetPassword {
		fmt.Println("🔑 Resetting postgres superuser password...")

		// Prompt for password if not provided
		if newPassword == "" {
			fmt.Print("Enter new password for postgres superuser: ")
			// Read password from stdin (without echoing)
			fmt.Scanln(&newPassword)
			if newPassword == "" {
				return fmt.Errorf("password cannot be empty")
			}
		}

		sensitivePassword := utils.NewSensitiveString(newPassword)
		if err := postgres.ResetPassword(sensitivePassword); err != nil {
			return fmt.Errorf("failed to reset password: %w", err)
		}
		fmt.Println("✅ Password reset completed successfully")
	}

	// // Step 5: Start PostgreSQL
	// fmt.Println("🚀 Starting PostgreSQL...")
	// if err := postgres.Start(); err != nil {
	// 	return fmt.Errorf("failed to start PostgreSQL: %w", err)
	// }

	fmt.Println("✅ Ready to start")
	return nil
}

// createVersionCommand creates the version command
func createVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("postgres-cli version %s\n", version)
			if verbose {
				fmt.Printf("Binary directory: %s\n", binDir)
				fmt.Printf("Data directory: %s\n", dataDir)
				if detectedDirs != nil {
					fmt.Printf("Auto-detection: successful\n")
				} else {
					fmt.Printf("Auto-detection: failed\n")
				}
			}
		},
	}
}

// Helper function to get effective data directory
func getDataDir() string {
	if dataDir != "" {
		return dataDir
	}
	if detectedDirs != nil {
		return detectedDirs.DataDir
	}
	return ""
}

// Helper function to get effective binary directory
func getBinDir() string {
	if binDir != "" {
		return binDir
	}
	if detectedDirs != nil {
		return detectedDirs.BinDir
	}
	return ""
}
