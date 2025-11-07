package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/flanksource/clicky"
	"github.com/samber/lo"
	"github.com/spf13/cobra"

	"github.com/flanksource/postgres/pkg"
	"github.com/flanksource/postgres/pkg/config"
	"github.com/flanksource/postgres/pkg/pgtune"
	"github.com/flanksource/postgres/pkg/server"
	"github.com/flanksource/postgres/pkg/utils"
)

var (
	postgres   server.Postgres
	createDb   string
	configFile string
	encoding   string
	locale     string
	authMethod string
	opts       pgtune.OptimizeOptions
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
	clicky.Infof("%s", GetVersionInfo())

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
				pconfig, err := config.LoadPostgresConf(configFile)
				if err != nil {
					return fmt.Errorf("failed to load postgres config file(%s): %w", configFile, err)
				}

				postgres.Config = pconfig
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

			if postgres.DryRun {
				clicky.Infof("📋 Dry run mode enabled")
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
	rootCmd.PersistentFlags().BoolVar(&postgres.DryRun, "dry-run", false, "Enable dry-run mode to simulate actions without making changes")
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Configuration file path")
	rootCmd.PersistentFlags().StringVar(&locale, "locale", "C", "Locale for initialized database")
	rootCmd.PersistentFlags().StringVar(&encoding, "encoding", "UTF8", "Encoding for initialized database")

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
  postgres-cli auto-start                           Start PostgreSQL normally
  postgres-cli auto-start --auto-init               Initialize and start if needed
  postgres-cli auto-start --pg-tune                 Optimize config before starting
  postgres-cli auto-start --auto-upgrade            Upgrade if needed, then start
  postgres-cli auto-start --auto-reset-password     Reset password, then start
  postgres-cli auto-start --auto-init --pg-tune     Initialize, optimize, then start
  postgres-cli auto-start --dry-run                 Validate permissions without starting`,
		RunE: runAutoStart,
	}

	cmd.Flags().IntVar(&opts.MaxConnections, "max-connections", getIntVar("PG_TUNE_MAX_CONNECTIONS"), "Max connections for pg_tune (0 = auto-calculate)")
	cmd.Flags().IntVar(&opts.MemoryMB, "memory", getIntVar("PG_TUNE_MEMORY"), "Override detected memory in MB for pg_tune")
	cmd.Flags().IntVar(&opts.Cores, "cpus", getIntVar("PG_TUNE_CPUS"), "Override detected CPU count for pg_tune")
	cmd.Flags().StringVar(&opts.DBType, "type", "web", "Database type for pg_tune: web, oltp, dw, desktop, mixed")
	cmd.Flags().StringVar(&authMethod, "auth-method", lo.CoalesceOrEmpty(os.Getenv("PG_AUTH_METHOD"), string(pkg.AuthScramSHA)), "Authentication method for pg_hba.conf (auto-detected if not specified)")
	cmd.Flags().BoolVar(&opts.Enabled, "pg-tune", true, "Run pg_tune to optimize postgresql.conf before starting")
	cmd.Flags().Bool("auto-upgrade", true, "Automatically upgrade PostgreSQL if version mismatch detected")
	cmd.Flags().Bool("auto-reset-password", false, "Reset postgres superuser password on start")
	cmd.Flags().Bool("auto-init", true, "Automatically initialize database if data directory doesn't exist")
	cmd.Flags().Int("upgrade-to", 0, "Target PostgreSQL version for upgrade (default: auto-detect latest)")

	return cmd
}

// runAutoStart handles the auto-start command execution
func runAutoStart(cmd *cobra.Command, args []string) error {
	autoInit, _ := cmd.Flags().GetBool("auto-init")
	autoUpgrade, _ := cmd.Flags().GetBool("auto-upgrade")
	autoResetPassword, _ := cmd.Flags().GetBool("auto-reset-password")
	upgradeTo, _ := cmd.Flags().GetInt("upgrade-to")

	// Log current user context
	uid, gid, username, err := utils.GetCurrentUserInfo()
	if err != nil {
		return fmt.Errorf("failed to get user info: %w", err)
	}

	clicky.Infof("Running auto-start with: %s", clicky.Map(map[string]any{
		"auto-init":           autoInit,
		"pg-tune":             opts.Enabled,
		"auto-upgrade":        autoUpgrade,
		"auto-reset-password": autoResetPassword,
		"upgrade-to":          upgradeTo,
		"database":            postgres.Database,
		"uid":                 uid,
		"gid":                 gid,
		"username":            username,
	}))

	// Check if already running
	if !postgres.DryRun && postgres.IsRunning() {
		clicky.Infof("PostgreSQL is already running")
		return nil
	}

	// Step 1: Auto-init if data directory doesn't exist
	if !postgres.Exists() {
		if autoInit {

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
			if err := postgres.Upgrade(targetVersion); err != nil {
				return fmt.Errorf("failed to upgrade PostgreSQL: %w", err)
			}
		} else {
			fmt.Printf("✅ PostgreSQL is already at version %d (target: %d)\n", currentVersion, targetVersion)
		}
	}

	// Step 3: Run pg_tune if requested
	if opts.Enabled {

		content, err := pgtune.OptimizeAndSave(opts)
		if err != nil {
			return fmt.Errorf("failed to run pg_tune: %w", err)
		}
		if postgres.DryRun {
			clicky.Infof("📄 Generated postgresql.tune.conf by pg_tune:")

			fmt.Println(clicky.CodeBlock("properties", content).ANSI())
		} else {
			configPath := filepath.Join(postgres.DataDir, "postgresql.tune.conf")
			if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
				return fmt.Errorf("failed to write postgresql.tune.conf: %w", err)
			}
			clicky.Infof("✅ pg_tune optimization applied and saved to %s", configPath)

			// Ensure postgresql.conf includes the tune file
			postgresConfPath := filepath.Join(postgres.DataDir, "postgresql.conf")
			if err := config.EnsureIncludeDirective(postgresConfPath, "postgresql.tune.conf"); err != nil {
				return fmt.Errorf("failed to update postgresql.conf: %w", err)
			}
			clicky.Infof("✅ postgresql.conf updated to include postgresql.tune.conf")
		}
	}

	// Step 4: Reset password if requested
	if autoResetPassword {

		newPassword, err := utils.FileEnv("POSTGRES_PASSWORD", "")
		if err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}
		sensitivePassword := utils.NewSensitiveString(newPassword)
		if err := postgres.ResetPassword(sensitivePassword); err != nil {
			return fmt.Errorf("failed to reset password: %w", err)
		}

	}

	if createDb != "" {

		if err := postgres.CreateDatabase(createDb); err != nil {
			//FIXME
			fmt.Printf("❌ Failed to create database '%s': %v\n", createDb, err)
			// return fmt.Errorf("failed to create database '%s': %w", createDb, err)
		}
	}

	if err := postgres.SetupPgHBA(authMethod); err != nil {
		return fmt.Errorf("failed to setup pg_hba.conf: %w", err)
	}
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
