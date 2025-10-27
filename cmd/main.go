package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/flanksource/clicky"
	"github.com/flanksource/postgres/pkg/pgtune"
	"github.com/flanksource/postgres/pkg/utils"
)

const version = "1.0.0"

var (
	// Global flags
	dataDir    string
	binDir     string
	verbose    bool
	createDb   string
	configFile string
	encoding   string
	locale     string
	opts       pgtune.OptimizeOptions
	// Auto-detected directories
	detectedDirs *utils.PostgreSQLDirs
)

func getIntVar(name string) int {
	val := os.Getenv(name)
	if val == "" {
		return 0
	}
	result, _ := strconv.Atoi(val)
	return result
}

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
	rootCmd.PersistentFlags().StringVar(&locale, "locale", "C", "Locale for initialized database")
	rootCmd.PersistentFlags().StringVar(&encoding, "encoding", "UTF8", "Encoding for initialized database")
	rootCmd.PersistentFlags().BoolVar(&opts.Enabled, "pg-tune", os.Getenv("PG_TUNE") == "true", "Enable pg_tune optimization")
	rootCmd.PersistentFlags().IntVar(&opts.MaxConnections, "max-connections", getIntVar("PG_TUNE_MAX_CONNECTIONS"), "Max connections for pg_tune (0 = auto-calculate)")
	rootCmd.PersistentFlags().IntVar(&opts.MemoryMB, "memory", getIntVar("PG_TUNE_MEMORY"), "Override detected memory in MB for pg_tune")
	rootCmd.PersistentFlags().IntVar(&opts.Cores, "cpus", getIntVar("PG_TUNE_CPUS"), "Override detected CPU count for pg_tune")
	rootCmd.PersistentFlags().StringVar(&opts.DBType, "type", "web", "Database type for pg_tune: web, oltp, dw, desktop, mixed")
	rootCmd.PersistentFlags().StringVar(&createDb, "create-db", "postgres", "Database name to create on initialization")

	clicky.BindAllFlags(rootCmd.PersistentFlags())

	// Add command groups
	rootCmd.AddCommand(
		createServerCommands(),
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
  pgconfig auto-start --auto-init --pg-tune     Initialize, optimize, then start
  pgconfig auto-start --dry-run                 Validate permissions without starting`,
		RunE: runAutoStart,
	}

	cmd.Flags().Bool("auto-init", false, "Automatically initialize database if data directory doesn't exist")
	cmd.Flags().Bool("pg-tune", false, "Run pg_tune to optimize postgresql.conf before starting")
	cmd.Flags().Bool("auto-upgrade", false, "Automatically upgrade PostgreSQL if version mismatch detected")
	cmd.Flags().Bool("auto-reset-password", false, "Reset postgres superuser password on start")
	cmd.Flags().Int("upgrade-to", 0, "Target PostgreSQL version for upgrade (default: auto-detect latest)")
	cmd.Flags().String("new-password", "", "New password for auto-reset (prompted if not provided)")
	cmd.Flags().Bool("dry-run", false, "Validate permissions and configuration without making changes")

	return cmd
}

// runAutoStart handles the auto-start command execution
func runAutoStart(cmd *cobra.Command, args []string) error {
	autoInit, _ := cmd.Flags().GetBool("auto-init")
	autoUpgrade, _ := cmd.Flags().GetBool("auto-upgrade")
	autoResetPassword, _ := cmd.Flags().GetBool("auto-reset-password")
	upgradeTo, _ := cmd.Flags().GetInt("upgrade-to")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	// Log current user context
	uid, gid, username, err := utils.GetCurrentUserInfo()
	if err != nil {
		return fmt.Errorf("failed to get user info: %w", err)
	}
	fmt.Printf("Running as user: %s (UID: %d, GID: %d)\n", username, uid, gid)

	// Perform pre-flight permission checks
	dataDir := getDataDir()
	if dataDir == "" {
		return fmt.Errorf("data directory not detected, use --data-dir flag")
	}

	fmt.Printf("Checking permissions for PGDATA: %s\n", dataDir)
	if err := utils.CheckPGDATAPermissions(dataDir); err != nil {
		if permErr, ok := err.(*utils.PermissionError); ok {
			// Exit with code 126 (permission denied)
			fmt.Fprintf(os.Stderr, "\n❌ %s\n", permErr.Error())
			os.Exit(126)
		}
		// For non-directory-exists errors during dry-run, just warn
		if !os.IsNotExist(err) || !dryRun {
			return fmt.Errorf("permission check failed: %w", err)
		}
	}

	fmt.Println("✅ Permission checks passed")

	// If dry-run mode, exit successfully
	if dryRun {
		fmt.Println("\n✅ Dry-run validation completed successfully")
		fmt.Println("All permission checks passed. Ready to start PostgreSQL.")
		return nil
	}

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
	if opts.Enabled {
		fmt.Println("🔧 Running pg_tune to optimize configuration...")

		opts.DataDir = getDataDir()
		if opts.DataDir == "" {
			return fmt.Errorf("data directory not detected")
		}

		err := pgtune.OptimizeAndSave(opts)
		if err != nil {
			return fmt.Errorf("failed to run pg_tune: %w", err)
		}

		fmt.Println("✅ Configuration optimized successfully")
	}

	// Step 4: Reset password if requested
	if autoResetPassword && os.Getenv("POSTGRES_PASSWORD") != "" {
		fmt.Println("🔑 Resetting postgres superuser password from POSTGRES_PASSWORD")

		newPassword := os.Getenv("POSTGRES_PASSWORD")
		sensitivePassword := utils.NewSensitiveString(newPassword)
		if err := postgres.ResetPassword(sensitivePassword); err != nil {
			return fmt.Errorf("failed to reset password: %w", err)
		}
		fmt.Println("✅ Password reset completed successfully")
	}

	if createDb != "" {
		fmt.Printf("🛠️  Ensuring database '%s' exists...\n", createDb)
		if err := postgres.CreateDatabase(createDb); err != nil {
			return fmt.Errorf("failed to create database '%s': %w", createDb, err)
		}
		fmt.Printf("✅ Database '%s' is ready\n", createDb)
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
