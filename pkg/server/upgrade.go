package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/api/icons"
	"github.com/flanksource/clicky/exec"
	"github.com/flanksource/postgres/pkg/config"
)

func (p *Postgres) Upgrade(targetVersion int) error {

	// Detect current version
	currentVersion, err := p.DetectVersion()
	if err != nil {
		return fmt.Errorf("failed to detect current PostgreSQL version: %w", err)
	}

	// Validate versions
	if currentVersion >= targetVersion {
		fmt.Printf("✅ PostgreSQL %d is already at or above target version %d\n", currentVersion, targetVersion)
		return nil
	}

	fmt.Printf("🚀 Starting PostgreSQL upgrade process from 🔍  %d to 🎯 %d...\n", currentVersion, targetVersion)

	if currentVersion < 14 || targetVersion > 17 {
		return fmt.Errorf("invalid version range. Current: %d, Target: %d. Supported versions: 14-17", currentVersion, targetVersion)
	}

	// Check if data exists
	if !p.Exists() {
		return fmt.Errorf("PostgreSQL data directory does not exist at %s", p.DataDir)
	}

	// Ensure PostgreSQL is stopped before upgrade
	if p.IsRunning() {
		fmt.Println("🛑 Stopping PostgreSQL for upgrade...")
		if err := p.Stop(); err != nil {
			return fmt.Errorf("failed to stop PostgreSQL before upgrade: %w", err)
		}
	}

	// Setup backup directory structure
	backupDir := filepath.Join(p.DataDir, "backups")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Backup current data
	originalBackupPath := filepath.Join(backupDir, fmt.Sprintf("data-%d", currentVersion))
	fmt.Printf("📦 Backing up current data to %s...\n", originalBackupPath)

	if err := p.backupDataDirectory(originalBackupPath); err != nil {
		return fmt.Errorf("failed to backup data directory: %w", err)
	}

	// Perform sequential upgrades
	for version := currentVersion; version < targetVersion; version++ {
		nextVersion := version + 1
		fmt.Println(clicky.Text("").Add(icons.ArrowUp).Append(" Upgrading Postgres from", "font-bold text-red-500").Append(version).Append("to").Append(nextVersion).String())

		if err := p.upgradeSingle(version, nextVersion); err != nil {
			return fmt.Errorf("upgrade from %d to %d failed: %w", version, nextVersion, err)
		}
	}

	// Update binary directory for new version
	p.BinDir = p.resolveBinDir(targetVersion)

	fmt.Printf("\n🎉 All upgrades completed successfully!\n")
	fmt.Printf("✅ Final version: PostgreSQL %d\n", targetVersion)
	fmt.Printf("💾 Original data preserved in %s\n", originalBackupPath)

	return nil
}

// runPgUpgrade executes the pg_upgrade command
func (p *Postgres) runPgUpgrade(oldBinDir, newBinDir, oldDataDir, newDataDir string) error {
	// Create socket directory
	socketDir := "/var/run/postgresql"
	if err := os.MkdirAll(socketDir, 0755); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}

	// Change to parent directory for pg_upgrade
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	if err := os.Chdir(filepath.Dir(p.DataDir)); err != nil {
		return fmt.Errorf("failed to change directory: %w", err)
	}
	defer os.Chdir(originalDir)

	// Run compatibility check first
	// Use --socketdir to explicitly control where pg_upgrade creates Unix sockets
	// This is critical when running as postgres user to avoid permission issues
	checkArgs := []string{
		"--old-bindir=" + oldBinDir,
		"--new-bindir=" + newBinDir,
		"--old-datadir=" + oldDataDir,
		"--new-datadir=" + newDataDir,
		"--socketdir=" + socketDir,
		"--check",
	}

	fmt.Println("Checking cluster compatibility...")
	fmt.Println("pg_upgrade check args:", strings.Join(checkArgs, "\n"))
	checkProcess := clicky.Exec(filepath.Join(newBinDir, "pg_upgrade"), checkArgs...).Run()
	if checkProcess.Err != nil {
		return fmt.Errorf("pg_upgrade compatibility check failed: %w, output: %s", checkProcess.Err, checkProcess.Out())
	}

	// Run the actual upgrade
	upgradeArgs := []string{
		"--old-bindir=" + oldBinDir,
		"--new-bindir=" + newBinDir,
		"--old-datadir=" + oldDataDir,
		"--new-datadir=" + newDataDir,
		"--socketdir=" + socketDir,
	}

	fmt.Println("Performing upgrade...")
	upgradeProcess := clicky.Exec(filepath.Join(newBinDir, "pg_upgrade"), upgradeArgs...).Run()

	// if !upgradeProcess.IsOk() {
	fmt.Println(upgradeProcess.Pretty().ANSI())
	fmt.Println(upgradeProcess.Out())

	// }
	p.lastStdout = upgradeProcess.GetStdout()
	p.lastStderr = upgradeProcess.GetStderr()

	if upgradeProcess.Err != nil {
		return fmt.Errorf("pg_upgrade failed: %w, output: %s", upgradeProcess.Err, upgradeProcess.Out())
	}

	return nil
}

// moveUpgradedData moves the upgraded data from upgrade directory to main data directory
func (p *Postgres) moveUpgradedData(newDataDir string) error {
	// Remove all files from main data directory except backups and upgrades
	entries, err := os.ReadDir(p.DataDir)
	if err != nil {
		return fmt.Errorf("failed to read data directory: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if name == "backups" || name == "upgrades" {
			continue
		}

		path := filepath.Join(p.DataDir, name)
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("failed to remove %s: %w", path, err)
		}
	}

	// Move all files from new data directory to main data directory
	newEntries, err := os.ReadDir(newDataDir)
	if err != nil {
		return fmt.Errorf("failed to read new data directory: %w", err)
	}

	for _, entry := range newEntries {
		name := entry.Name()
		sourcePath := filepath.Join(newDataDir, name)
		destPath := filepath.Join(p.DataDir, name)

		if err := os.Rename(sourcePath, destPath); err != nil {
			return fmt.Errorf("failed to move %s: %w", name, err)
		}
	}

	// Clean up the upgrade directory
	if err := os.RemoveAll(filepath.Dir(newDataDir)); err != nil {
		fmt.Printf("Warning: failed to clean up upgrade directory: %v\n", err)
	}

	return nil
}

func (p *Postgres) GetConf() config.Conf {
	auto, _ := config.LoadConfFile(filepath.Join(p.DataDir, "postgres.auto.conf"))
	config, _ := config.LoadConfFile(filepath.Join(p.DataDir, "postgresql.conf"))
	return auto.MergeFrom(config)
}

func (p *Postgres) Psql(args ...string) *exec.Process {
	if err := p.ensureBinDir(); err != nil {
		panic(fmt.Errorf("failed to resolve binary directory: %w", err))
	}
	if p.DataDir == "" {
		panic("DataDir not specified")
	}
	cmd := clicky.Exec(filepath.Join(p.BinDir, "psql"), args...)
	return cmd.Debug()
}

// backupDataDirectory creates a backup of the current data directory
func (p *Postgres) backupDataDirectory(backupPath string) error {
	if err := os.MkdirAll(backupPath, 0750); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	clicky.Exec("cp", "-a", filepath.Join(p.DataDir, "main"), backupPath)

	return nil
}

// upgradeSingle performs a single version upgrade (e.g., 14 -> 15)
func (p *Postgres) upgradeSingle(fromVersion, toVersion int) error {
	oldBinDir := p.resolveBinDir(fromVersion)
	newBinDir := p.resolveBinDir(toVersion)
	upgradeDir := filepath.Join(p.DataDir, "upgrades")
	newDataDir := filepath.Join(upgradeDir, strconv.Itoa(toVersion))

	// Clean up any existing upgrade directory from previous failed attempts
	if _, err := os.Stat(newDataDir); err == nil {
		fmt.Printf("🧹 Cleaning up existing upgrade directory: %s\n", newDataDir)
		if err := os.RemoveAll(newDataDir); err != nil {
			return fmt.Errorf("failed to clean up existing upgrade directory: %w", err)
		}
	}

	fmt.Printf("🔍 Running pre-upgrade checks for PostgreSQL %d...\n", fromVersion)

	// Validate current cluster
	if err := p.validateCluster(oldBinDir, p.DataDir, fromVersion); err != nil {
		return fmt.Errorf("pre-upgrade validation failed: %w", err)
	}

	// Start old cluster temporarily to detect settings
	fmt.Printf("🔍 Detecting old cluster configuration...\n")
	oldServer := &Postgres{
		DataDir: p.DataDir,
		BinDir:  oldBinDir,
		Config:  p.Config,
	}

	if err := oldServer.Start(); err != nil {
		return fmt.Errorf("failed to start old cluster for detection: %w", err)
	}

	info, _ := oldServer.Info()

	fmt.Println(clicky.Text("📝 Detected old cluster information:", "font-bold").Append(clicky.MustFormat(info)).Append("-----------------"))

	// Get current configuration
	oldConf, err := oldServer.GetCurrentConf()
	if err != nil {
		oldServer.Stop()
		return fmt.Errorf("failed to detect old cluster configuration: %w", err)
	}

	// Stop old cluster
	if err := oldServer.Stop(); err != nil {
		return fmt.Errorf("failed to stop old cluster: %w", err)
	}

	// Extract initdb-applicable settings
	initdbConf := oldConf.ToConf().ForInitDB()
	fmt.Printf("✅ Detected initdb settings: %v\n", initdbConf)

	fmt.Printf("✅ Pre-upgrade checks completed for PostgreSQL %d\n", fromVersion)

	// Initialize new cluster with detected settings
	fmt.Printf("🔧 Initializing PostgreSQL %d cluster...\n", toVersion)
	if err := p.initNewClusterWithConf(newBinDir, newDataDir, initdbConf); err != nil {
		return fmt.Errorf("failed to initialize new cluster: %w", err)
	}

	// Run pg_upgrade
	fmt.Printf("⚡ Performing pg_upgrade from PostgreSQL %d to %d...\n", fromVersion, toVersion)
	if err := p.runPgUpgrade(oldBinDir, newBinDir, p.DataDir, newDataDir); err != nil {
		return fmt.Errorf("pg_upgrade failed: %w", err)
	}

	// Post-upgrade validation
	fmt.Printf("🔍 Running post-upgrade checks for PostgreSQL %d...\n", toVersion)
	if err := p.validateCluster(newBinDir, newDataDir, toVersion); err != nil {
		return fmt.Errorf("post-upgrade validation failed: %w", err)
	}

	// Move upgraded data to main location
	fmt.Printf("📦 Moving PostgreSQL %d data to main location...\n", toVersion)
	if err := p.moveUpgradedData(newDataDir); err != nil {
		return fmt.Errorf("failed to move upgraded data: %w", err)
	}

	fmt.Printf("✅ Upgrade from PostgreSQL %d to %d completed successfully!\n", fromVersion, toVersion)
	return nil
}
