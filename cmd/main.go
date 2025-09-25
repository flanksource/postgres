package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

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
			initializeDirectories()
		},
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", "", "PostgreSQL data directory (auto-detected if not specified)")
	rootCmd.PersistentFlags().StringVar(&binDir, "bin-dir", "", "PostgreSQL binary directory (auto-detected if not specified)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Configuration file path")

	// Add command groups
	rootCmd.AddCommand(
		createServerCommands(),
		createSchemaCommands(),
		createConfigCommands(),
		createVersionCommand(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
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
