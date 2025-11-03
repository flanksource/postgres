package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/flanksource/clicky"
)

// createServerCommands creates the server command group
func createServerCommands() *cobra.Command {
	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "PostgreSQL server management commands",
		Long:  "Commands for managing PostgreSQL server instances, including health checks, initialization, and maintenance",
	}

	// Add all server commands
	serverCmd.AddCommand(
		createHealthCommand(),
		createInitDBCommand(),
		createResetPasswordCommand(),
		createUpgradeCommand(),
		createBackupCommand(),
		createSQLCommand(),
		createStatusCommand(),
		createStartCommand(),
		createStopCommand(),
		createRestartCommand(),
	)

	return serverCmd
}

func createStopCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop PostgreSQL server",
		Long:  "Stop the PostgreSQL server gracefully",
		RunE: func(cmd *cobra.Command, args []string) error {

			if err := postgres.Stop(); err != nil {
				return fmt.Errorf("failed to stop PostgreSQL: %w", err)
			}
			fmt.Println("PostgreSQL server stopped successfully")
			return nil
		},
	}
}

func createStartCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start PostgreSQL server",
		Long:  "Start the PostgreSQL server",
		RunE: func(cmd *cobra.Command, args []string) error {

			if err := postgres.Start(); err != nil {
				return fmt.Errorf("failed to start PostgreSQL: %w", err)
			}
			fmt.Println("PostgreSQL server started successfully")
			return nil
		},
	}
}

func createRestartCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart PostgreSQL server",
		Long:  "Restart the PostgreSQL server gracefully",
		RunE: func(cmd *cobra.Command, args []string) error {

			if err := postgres.Stop(); err != nil {
				fmt.Println("Failed to stop postgres " + err.Error())
			}
			if err := postgres.Start(); err != nil {
				return fmt.Errorf("failed to restart PostgreSQL: %w", err)
			}
			fmt.Println("PostgreSQL server restarted successfully")
			return nil
		},
	}
}

// createHealthCommand creates the health command
func createHealthCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Perform comprehensive health check",
		Long:  "Perform a comprehensive health check of the PostgreSQL service",
		RunE: func(cmd *cobra.Command, args []string) error {

			if err := postgres.Health(); err != nil {
				return fmt.Errorf("health check failed: %w", err)
			}
			fmt.Println("PostgreSQL health check passed")
			return nil
		},
	}
}

// createInitDBCommand creates the initdb command
func createInitDBCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "initdb",
		Short: "Initialize PostgreSQL data directory",
		Long:  "Initialize a new PostgreSQL data directory with initdb",
		RunE: func(cmd *cobra.Command, args []string) error {

			if err := postgres.InitDB(); err != nil {
				return fmt.Errorf("failed to initialize database: %w", err)
			}
			fmt.Println("PostgreSQL data directory initialized successfully")
			return nil
		},
	}
}

// createResetPasswordCommand creates the reset-password command
func createResetPasswordCommand() *cobra.Command {
	resetPasswordCmd := &cobra.Command{
		Use:   "reset-password",
		Short: "Reset PostgreSQL password",
		Long:  "Reset the PostgreSQL superuser password",
		RunE: func(cmd *cobra.Command, args []string) error {
			if postgres.Password.IsEmpty() {
				return fmt.Errorf("password is required")
			}

			if err := postgres.ResetPassword(postgres.Password); err != nil {
				return fmt.Errorf("failed to reset password: %w", err)
			}
			return nil
		},
	}

	return resetPasswordCmd
}

// createUpgradeCommand creates the upgrade command
func createUpgradeCommand() *cobra.Command {
	upgradeCmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade PostgreSQL to target version",
		Long:  "Upgrade PostgreSQL data directory to the specified target version",
		RunE: func(cmd *cobra.Command, args []string) error {
			targetVersion, _ := cmd.Flags().GetInt("target-version")
			if targetVersion == 0 {
				return fmt.Errorf("target-version is required")
			}

			if err := postgres.Upgrade(targetVersion); err != nil {
				return fmt.Errorf("failed to upgrade: %w", err)
			}
			fmt.Printf("PostgreSQL upgraded to version %d successfully\n", targetVersion)
			return nil
		},
	}
	upgradeCmd.Flags().IntP("target-version", "t", 0, "Target PostgreSQL version (required)")
	upgradeCmd.MarkFlagRequired("target-version")
	return upgradeCmd
}

// createBackupCommand creates the backup command
func createBackupCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "backup",
		Short: "Create PostgreSQL backup",
		Long:  "Create a backup of the PostgreSQL instance using pg_dump",
		RunE: func(cmd *cobra.Command, args []string) error {

			if err := postgres.Backup(); err != nil {
				return fmt.Errorf("backup failed: %w", err)
			}
			fmt.Println("Backup completed successfully")
			return nil
		},
	}
}

// createSQLCommand creates the sql command
func createSQLCommand() *cobra.Command {
	sqlCmd := &cobra.Command{
		Use:   "sql",
		Short: "Execute SQL query",
		Long:  "Execute a SQL query and return results",
		RunE: func(cmd *cobra.Command, args []string) error {
			query, _ := cmd.Flags().GetString("query")
			file, _ := cmd.Flags().GetString("file")

			var sqlQuery string
			if query != "" {
				sqlQuery = query
			} else if file != "" {
				data, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("failed to read SQL file: %w", err)
				}
				sqlQuery = string(data)
			} else {
				return fmt.Errorf("either --query or --file must be specified")
			}

			results, err := postgres.SQL(sqlQuery)
			if err != nil {
				return fmt.Errorf("failed to execute SQL: %w", err)
			}

			fmt.Println(clicky.MustFormat(results))

			return nil
		},
	}
	sqlCmd.Flags().StringP("query", "q", "", "SQL query to execute")

	return sqlCmd
}

// createStatusCommand creates the status command
func createStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show comprehensive PostgreSQL status",
		Long:  "Show detailed status information about the PostgreSQL instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, _ := postgres.Info()
			fmt.Println("---")
			clicky.MustPrint(*info)
			fmt.Println("----")
			return nil
		},
	}
}
