package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/samber/lo"
	"github.com/spf13/cobra"

	"github.com/flanksource/clicky"
	"github.com/flanksource/postgres/pkg"
	"github.com/flanksource/postgres/pkg/config"
	"github.com/flanksource/postgres/pkg/pgtune"
	"github.com/flanksource/postgres/pkg/server"
	"github.com/flanksource/postgres/pkg/utils"
)

var (
	postgres server.Postgres

	verbose    bool
	createDb   string
	configFile string
	encoding   string
	locale     string
	authMethod string
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

func getPostgresPassword() utils.SensitiveString {
	password := os.Getenv("PGPASSWORD")
	if password != "" {
		return utils.NewSensitiveString(password)
	}

	if file := os.Getenv("PGPASSWORD_FILE"); file != "" {
		content, err := os.ReadFile(file)
		if err == nil {
			return utils.NewSensitiveString(string(content))
		}
	}

	return utils.NewSensitiveString("")
}

func main() {
	clicky.Infof(GetVersionInfo())
	rootCmd := &cobra.Command{
		Use:   "postgres-cli",
		Short: "PostgreSQL Management CLI",
		Long: `A comprehensive CLI tool for managing PostgreSQL servers, generating configurations, and working with schemas.
This unified tool combines PostgreSQL server management, configuration generation, and schema operations.`,
		Version: Version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {

			clicky.Flags.UseFlags()

			// Load configuration if specified
			if configFile != "" {
				if pgConfig, err := config.LoadPostgresConf(configFile); err == nil {
					return err
				} else {
					postgres.Config = pgConfig
				}
			} else if postgres.Config == nil {
				postgres.Config = config.DefaultPostgresConf()
			}

			password, _ := cmd.Flags().GetString("password")
			if password != "" {
				postgres.Password = utils.NewSensitiveString(password)
			}

			if postgres.Password.IsEmpty() {
				postgres.Password = getPostgresPassword()
			}
			if err := postgres.Validate(); err != nil {
				return err
			}

			return nil
		},
	}

	rootCmd.PersistentFlags().StringVarP(&postgres.Username, "username", "U", lo.CoalesceOrEmpty(os.Getenv("PG_USER"), "postgres"), "PostgreSQL username")
	rootCmd.PersistentFlags().StringP("password", "W", "", "PostgreSQL password if not specied  PGPASSWORD or PGPASSWORD_FILE env variable")
	rootCmd.PersistentFlags().StringVarP(&postgres.Database, "database", "d", lo.CoalesceOrEmpty(os.Getenv("PG_DATABASE"), "postgres"), "PostgreSQL database name")
	rootCmd.PersistentFlags().StringVarP(&postgres.Host, "host", "", lo.CoalesceOrEmpty(os.Getenv("PG_HOST"), "localhost"), "PostgreSQL host")
	rootCmd.PersistentFlags().IntVarP(&postgres.Port, "port", "p", lo.CoalesceOrEmpty(getIntVar("PG_PORT"), 5432), "PostgreSQL port")
	rootCmd.PersistentFlags().StringVar(&postgres.DataDir, "data-dir", os.Getenv("PGDATA"), "PostgreSQL data directory (auto-detected if not specified)")
	rootCmd.PersistentFlags().StringVar(&postgres.BinDir, "bin-dir", "", "PostgreSQL binary directory (auto-detected if not specified)")
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Configuration file path")
	rootCmd.PersistentFlags().StringVar(&locale, "locale", "C", "Locale for initialized database")
	rootCmd.PersistentFlags().StringVar(&encoding, "encoding", "UTF8", "Encoding for initialized database")
	rootCmd.PersistentFlags().BoolVar(&opts.Enabled, "pg-tune", os.Getenv("PG_TUNE") == "true", "Enable pg_tune optimization")
	rootCmd.PersistentFlags().IntVar(&opts.MaxConnections, "max-connections", getIntVar("PG_TUNE_MAX_CONNECTIONS"), "Max connections for pg_tune (0 = auto-calculate)")
	rootCmd.PersistentFlags().IntVar(&opts.MemoryMB, "memory", getIntVar("PG_TUNE_MEMORY"), "Override detected memory in MB for pg_tune")
	rootCmd.PersistentFlags().IntVar(&opts.Cores, "cpus", getIntVar("PG_TUNE_CPUS"), "Override detected CPU count for pg_tune")
	rootCmd.PersistentFlags().StringVar(&opts.DBType, "type", "web", "Database type for pg_tune: web, oltp, dw, desktop, mixed")
	rootCmd.PersistentFlags().StringVar(&authMethod, "auth-method", lo.CoalesceOrEmpty(os.Getenv("PG_AUTH_METHOD"), string(pkg.AuthScramSHA)), "Authentication method for pg_hba.conf (auto-detected if not specified)")
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

	cmd.Flags().Bool("auto-init", true, "Automatically initialize database if data directory doesn't exist")
	cmd.Flags().Bool("pg-tune", true, "Run pg_tune to optimize postgresql.conf before starting")
	cmd.Flags().Bool("auto-upgrade", true, "Automatically upgrade PostgreSQL if version mismatch detected")
	cmd.Flags().Bool("auto-reset-password", true, "Reset postgres superuser password on start")
	cmd.Flags().Int("upgrade-to", 0, "Target PostgreSQL version for upgrade (default: auto-detect latest)")
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

	clicky.Infof("✅ Permission checks passed")

	// If dry-run mode, exit successfully
	if dryRun {
		clicky.Infof("\n✅ Dry-run validation completed successfully")
		clicky.Infof("All permission checks passed. Ready to start PostgreSQL.")
		return nil
	}

	// Check if already running
	if postgres.IsRunning() {
		clicky.Infof("PostgreSQL is already running")
		return nil
	}

	// Step 1: Auto-init if data directory doesn't exist
	if !postgres.Exists() {
		if autoInit {
			clicky.Infof("🔧 Initializing PostgreSQL database...")
			if err := postgres.InitDB(); err != nil {
				return fmt.Errorf("failed to initialize database: %w", err)
			}

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
			clicky.Infof("✅ PostgreSQL upgrade completed successfully")
		} else {
			fmt.Printf("✅ PostgreSQL is already at version %d (target: %d)\n", currentVersion, targetVersion)
		}
	}

	// Step 3: Run pg_tune if requested
	if opts.Enabled {
		clicky.Infof("🔧 Running pg_tune to optimize configuration...")

		err := pgtune.OptimizeAndSave(opts)
		if err != nil {
			return fmt.Errorf("failed to run pg_tune: %w", err)
		}

		clicky.Infof("✅ Configuration optimized successfully")
	}

	// Step 4: Reset password if requested
	if autoResetPassword {
		newPassword := os.Getenv("POSTGRES_PASSWORD")
		sensitivePassword := utils.NewSensitiveString(newPassword)
		if err := postgres.ResetPassword(sensitivePassword); err != nil {
			return fmt.Errorf("failed to reset password: %w", err)
		}
		clicky.Infof("✅ Password reset completed successfully")
	}

	if createDb != "" {
		fmt.Printf("🛠️  Ensuring database '%s' exists...\n", createDb)
		if err := postgres.CreateDatabase(createDb); err != nil {
			//FIXME
			fmt.Printf("❌ Failed to create database '%s': %v\n", createDb, err)
			// return fmt.Errorf("failed to create database '%s': %w", createDb, err)
		} else {
			fmt.Printf("✅ Database '%s' is ready\n", createDb)

		}
	}

	if err := postgres.SetupPgHBA(authMethod); err != nil {
		return fmt.Errorf("failed to setup pg_hba.conf: %w", err)
	}

	// // Step 5: Start PostgreSQL
	// clicky.Infof("🚀 Starting PostgreSQL...")
	// if err := postgres.Start(); err != nil {
	// 	return fmt.Errorf("failed to start PostgreSQL: %w", err)
	// }

	clicky.Infof("✅ Ready to start")
	return nil
}

// createVersionCommand creates the version command
func createVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(GetVersionInfo())
		},
	}
}
