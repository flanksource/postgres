package server

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/flanksource/postgres/pkg"
	"github.com/flanksource/postgres/pkg/schemas"
	"github.com/flanksource/postgres/pkg/utils"
	_ "github.com/lib/pq"
)

type Postgres struct {
	Config     *pkg.PostgresConf
	DataDir    string
	BinDir     string // Auto-resolved based on detected version
	lastStdout string
	lastStderr string
}

// Use types from pkg package
type ExtensionInfo = pkg.ExtensionInfo
type ValidationError = pkg.ValidationError

// NewPostgres creates a new PostgreSQL service instance
func NewPostgres(config *pkg.PostgresConf, dataDir string) *Postgres {
	return &Postgres{
		Config:  config,
		DataDir: dataDir,
	}
}

// Note: For embedded postgres functionality, use pkg/embedded.NewEmbeddedPostgres() instead

// DescribeConfig executes `postgres --describe-config` and returns parsed parameters
func (p *Postgres) DescribeConfig() ([]schemas.Param, error) {
	if p.BinDir == "" {
		return nil, fmt.Errorf("postgres binary directory not set")
	}

	postgresPath := filepath.Join(p.BinDir, "postgres")

	// Execute postgres --describe-config
	cmd := exec.Command(postgresPath, "--describe-config")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run postgres --describe-config: %w", err)
	}

	return schemas.ParseDescribeConfig(string(output))
}

// DetectVersion reads the PostgreSQL version from the data directory
func (p *Postgres) DetectVersion() (int, error) {
	if p.DataDir == "" {
		return 0, fmt.Errorf("data directory not specified")
	}

	versionFile := filepath.Join(p.DataDir, "PG_VERSION")
	content, err := os.ReadFile(versionFile)
	if err != nil {
		return 0, fmt.Errorf("failed to read PG_VERSION file: %w", err)
	}

	versionStr := strings.TrimSpace(string(content))
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		return 0, fmt.Errorf("invalid version format in PG_VERSION: %s", versionStr)
	}

	return version, nil
}

// resolveBinDir returns the binary directory for a specific PostgreSQL version
func (p *Postgres) resolveBinDir(version int) string {
	return fmt.Sprintf("/usr/lib/postgresql/%d/bin", version)
}

// ensureBinDir sets the BinDir based on detected version if not already set
func (p *Postgres) ensureBinDir() error {
	if p.BinDir != "" {
		return nil
	}

	version, err := p.DetectVersion()
	if err != nil {
		return err
	}

	p.BinDir = p.resolveBinDir(version)
	return nil
}

// Health performs a comprehensive health check of the PostgreSQL service
func (p *Postgres) Health() error {
	if p == nil {
		return fmt.Errorf("PostgreSQL service is nil")
	}
	if err := p.ensureBinDir(); err != nil {
		return fmt.Errorf("failed to resolve binary directory: %w", err)
	}

	// Check if PostgreSQL is running
	if !p.IsRunning() {
		return fmt.Errorf("PostgreSQL is not running")
	}

	// Use pg_isready to check if server is accepting connections
	host := "localhost"
	if p.Config != nil && p.Config.ListenAddresses != "" && p.Config.ListenAddresses != "*" {
		host = p.Config.ListenAddresses
	}

	port := 5432
	if p.Config != nil && p.Config.Port != 0 {
		port = p.Config.Port
	}

	cmd := exec.Command(
		filepath.Join(p.BinDir, "pg_isready"),
		"-h", host,
		"-p", strconv.Itoa(port),
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_isready failed: %w", err)
	}

	// Additional health checks can be added here
	// - Check data directory permissions
	// - Verify configuration files
	// - Check disk space

	return nil
}

func (p *Postgres) IsRunning() bool {
	if p.DataDir == "" {
		return false
	}

	pidFilePath := filepath.Join(p.DataDir, "postmaster.pid")
	if _, err := os.Stat(pidFilePath); os.IsNotExist(err) {
		return false
	}

	pidBytes, err := os.ReadFile(pidFilePath)
	if err != nil {
		return false
	}

	lines := strings.Split(strings.TrimSpace(string(pidBytes)), "\n")
	if len(lines) == 0 {
		return false
	}

	pid, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	return err == nil
}

type PGVersion = pkg.PgVersion

func (p *Postgres) GetVersion() PGVersion {
	if err := p.ensureBinDir(); err != nil {
		return ""
	}

	cmd := exec.Command(filepath.Join(p.BinDir, "postgres"), "--version")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	versionStr := string(output)
	re := regexp.MustCompile(`PostgreSQL (\d+\.\d+(?:\.\d+)?)`)
	matches := re.FindStringSubmatch(versionStr)
	if len(matches) < 2 {
		return ""
	}

	return PGVersion(matches[1])
}

func (p *Postgres) Exists() bool {
	if p.DataDir == "" {
		return false
	}

	pgVersionFile := filepath.Join(p.DataDir, "PG_VERSION")
	if _, err := os.Stat(pgVersionFile); os.IsNotExist(err) {
		return false
	}

	postgresConf := filepath.Join(p.DataDir, "postgresql.conf")
	if _, err := os.Stat(postgresConf); os.IsNotExist(err) {
		return false
	}

	return true
}

func (p *Postgres) SQL(sqlQuery string) ([]map[string]interface{}, error) {
	if p == nil {
		return nil, fmt.Errorf("PostgreSQL service is nil")
	}

	// Default connection parameters
	host := "localhost"
	port := 5432
	user := "postgres"
	database := "postgres"
	var password utils.SensitiveString

	// Use config values if available
	if p.Config != nil {
		if p.Config.ListenAddresses != "" && p.Config.ListenAddresses != "*" {
			host = p.Config.ListenAddresses
		}
		if p.Config.Port != 0 {
			port = p.Config.Port
		}
		// No SuperuserPassword field available in PostgresConf
	}

	connStr := fmt.Sprintf("host=%s port=%d user=%s dbname=%s sslmode=disable",
		host, port, user, database)

	if !password.IsEmpty() {
		connStr += " password=" + password.Value()
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	rows, err := db.Query(sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var results []map[string]interface{}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))

		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		row := make(map[string]interface{})
		for i, column := range columns {
			row[column] = values[i]
		}

		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return results, nil
}

func (p *Postgres) Backup() error {
	if p.BinDir == "" {
		return fmt.Errorf("BinDir not specified")
	}

	host := "localhost"
	port := 5432
	user := "postgres"
	database := "postgres"

	// Use config values if available
	if p.Config != nil {
		if p.Config.ListenAddresses != "" && p.Config.ListenAddresses != "*" && p.Config.ListenAddresses != "localhost" {
			host = p.Config.ListenAddresses
		}
		if p.Config.Port != 0 {
			port = p.Config.Port
		}
	}

	backupFile := fmt.Sprintf("%s_backup_%s.sql", database, time.Now().Format("20060102_150405"))

	args := []string{
		"-h", host,
		"-p", strconv.Itoa(port),
		"-U", user,
		"-d", database,
		"-f", backupFile,
		"--verbose",
		"--no-password",
	}

	cmd := exec.Command(filepath.Join(p.BinDir, "pg_dump"), args...)

	// Only set password for non-localhost connections or if explicitly provided
	// No SuperuserPassword field available in PostgresConf

	output, err := cmd.CombinedOutput()
	p.lastStdout = string(output)
	p.lastStderr = string(output)

	if err != nil {
		return fmt.Errorf("backup failed: %w, output: %s", err, string(output))
	}

	return nil
}

func (p *Postgres) InitDB() error {
	if p.BinDir == "" {
		return fmt.Errorf("BinDir not specified")
	}

	if p.DataDir == "" {
		return fmt.Errorf("DataDir not specified")
	}

	if _, err := os.Stat(p.DataDir); !os.IsNotExist(err) {
		return fmt.Errorf("data directory already exists: %s", p.DataDir)
	}

	if err := os.MkdirAll(p.DataDir, 0700); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	args := []string{
		"-D", p.DataDir,
		"-A", "trust", // Use trust authentication for localhost by default
		"--locale=C",
		"--encoding=UTF8",
		"-U", "postgres", // Always use postgres superuser
	}

	cmd := exec.Command(filepath.Join(p.BinDir, "initdb"), args...)

	// Generally no password needed for initdb with trust auth
	// No SuperuserPassword field available in PostgresConf

	output, err := cmd.CombinedOutput()
	p.lastStdout = string(output)
	p.lastStderr = string(output)

	if err != nil {
		return fmt.Errorf("initdb failed: %w, output: %s", err, string(output))
	}

	return nil
}

// ResetPassword resets the PostgreSQL superuser password using a temporary server instance
func (p *Postgres) ResetPassword(newPassword utils.SensitiveString) error {
	if newPassword.IsEmpty() {
		return fmt.Errorf("new password not specified")
	}

	if err := p.ensureBinDir(); err != nil {
		return fmt.Errorf("failed to resolve binary directory: %w", err)
	}

	// Check if data directory exists
	if !p.Exists() {
		return fmt.Errorf("PostgreSQL data directory does not exist at %s", p.DataDir)
	}

	// Check if PostgreSQL is already running
	wasRunning := p.IsRunning()
	if wasRunning {
		return fmt.Errorf("PostgreSQL is currently running, stop it first before resetting password")
	}

	// Start PostgreSQL temporarily on alternate port for password reset
	tempPort := 5433
	logFile := filepath.Join("/tmp", "postgres-reset.log")

	fmt.Printf("🔑 Starting PostgreSQL temporarily on port %d for password reset...\n", tempPort)

	startArgs := []string{
		"-D", p.DataDir,
		"-l", logFile,
		"-o", fmt.Sprintf("-p %d", tempPort),
		"start",
	}

	startCmd := exec.Command(filepath.Join(p.BinDir, "pg_ctl"), startArgs...)
	startOutput, err := startCmd.CombinedOutput()
	p.lastStdout = string(startOutput)
	p.lastStderr = string(startOutput)

	if err != nil {
		return fmt.Errorf("failed to start PostgreSQL for password reset: %w, output: %s", err, string(startOutput))
	}

	// Wait a moment for PostgreSQL to start
	time.Sleep(3 * time.Second)

	// Reset the password
	user := "postgres"
	resetSQL := fmt.Sprintf("ALTER USER %s PASSWORD '%s';", user, newPassword.Value())

	fmt.Printf("🔑 Resetting password for user %s...\n", user)

	psqlCmd := exec.Command(
		filepath.Join(p.BinDir, "psql"),
		"-p", strconv.Itoa(tempPort),
		"-c", resetSQL,
	)

	psqlOutput, err := psqlCmd.CombinedOutput()
	if err != nil {
		// Make sure to stop PostgreSQL even if password reset fails
		p.stopTempPostgreSQL()
		return fmt.Errorf("failed to reset password: %w, output: %s", err, string(psqlOutput))
	}

	fmt.Println("✅ Password reset completed successfully")

	// Stop the temporary PostgreSQL instance
	fmt.Println("🛑 Stopping temporary PostgreSQL instance...")
	if err := p.stopTempPostgreSQL(); err != nil {
		return fmt.Errorf("password reset succeeded but failed to stop temporary PostgreSQL: %w", err)
	}

	fmt.Println("✅ Password reset process completed")
	return nil
}

func (p *Postgres) Upgrade(targetVersion int) error {
	fmt.Printf("🚀 Starting PostgreSQL upgrade process...\n")

	// Detect current version
	currentVersion, err := p.DetectVersion()
	if err != nil {
		return fmt.Errorf("failed to detect current PostgreSQL version: %w", err)
	}

	fmt.Printf("🔍 Current PostgreSQL version: %d\n", currentVersion)
	fmt.Printf("🎯 Target PostgreSQL version: %d\n", targetVersion)

	// Validate versions
	if currentVersion >= targetVersion {
		fmt.Printf("✅ PostgreSQL %d is already at or above target version %d\n", currentVersion, targetVersion)
		return nil
	}

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
		fmt.Printf("\n========================================\n")
		fmt.Printf("Upgrading PostgreSQL from %d to %d\n", version, nextVersion)
		fmt.Printf("========================================\n")

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

// backupDataDirectory creates a backup of the current data directory
func (p *Postgres) backupDataDirectory(backupPath string) error {
	if err := os.MkdirAll(backupPath, 0750); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Copy all contents except backups and upgrades directories
	entries, err := os.ReadDir(p.DataDir)
	if err != nil {
		return fmt.Errorf("failed to read data directory: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if name == "backups" || name == "upgrades" {
			continue // Skip these directories
		}

		sourcePath := filepath.Join(p.DataDir, name)
		destPath := filepath.Join(backupPath, name)

		if entry.IsDir() {
			if err := p.copyDir(sourcePath, destPath); err != nil {
				return fmt.Errorf("failed to copy directory %s: %w", name, err)
			}
		} else {
			if err := p.copyFile(sourcePath, destPath); err != nil {
				return fmt.Errorf("failed to copy file %s: %w", name, err)
			}
		}
	}

	return nil
}

// upgradeSingle performs a single version upgrade (e.g., 14 -> 15)
func (p *Postgres) upgradeSingle(fromVersion, toVersion int) error {
	oldBinDir := p.resolveBinDir(fromVersion)
	newBinDir := p.resolveBinDir(toVersion)
	upgradeDir := filepath.Join(p.DataDir, "upgrades")
	newDataDir := filepath.Join(upgradeDir, strconv.Itoa(toVersion))

	fmt.Printf("🔍 Running pre-upgrade checks for PostgreSQL %d...\n", fromVersion)

	// Validate current cluster
	if err := p.validateCluster(oldBinDir, p.DataDir, fromVersion); err != nil {
		return fmt.Errorf("pre-upgrade validation failed: %w", err)
	}

	fmt.Printf("✅ Pre-upgrade checks completed for PostgreSQL %d\n", fromVersion)

	// Initialize new cluster
	fmt.Printf("🔧 Initializing PostgreSQL %d cluster...\n", toVersion)
	if err := p.initNewCluster(newBinDir, newDataDir); err != nil {
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

// validateCluster validates a PostgreSQL cluster
func (p *Postgres) validateCluster(binDir, dataDir string, expectedVersion int) error {
	// Check PG_VERSION file
	versionFile := filepath.Join(dataDir, "PG_VERSION")
	if _, err := os.Stat(versionFile); os.IsNotExist(err) {
		return fmt.Errorf("PG_VERSION file not found at %s", versionFile)
	}

	content, err := os.ReadFile(versionFile)
	if err != nil {
		return fmt.Errorf("failed to read PG_VERSION: %w", err)
	}

	version, err := strconv.Atoi(strings.TrimSpace(string(content)))
	if err != nil {
		return fmt.Errorf("invalid version format: %s", string(content))
	}

	if version != expectedVersion {
		return fmt.Errorf("expected PostgreSQL %d, but found version %d", expectedVersion, version)
	}

	// Check control data
	cmd := exec.Command(filepath.Join(binDir, "pg_controldata"), dataDir)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to read control data: %w", err)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "Database cluster state") {
		return fmt.Errorf("invalid control data output")
	}

	return nil
}

// initNewCluster initializes a new PostgreSQL cluster
func (p *Postgres) initNewCluster(binDir, dataDir string) error {
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	cmd := exec.Command(filepath.Join(binDir, "initdb"), "-D", dataDir)
	output, err := cmd.CombinedOutput()
	p.lastStdout = string(output)
	p.lastStderr = string(output)

	if err != nil {
		return fmt.Errorf("initdb failed: %w, output: %s", err, string(output))
	}

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
	checkArgs := []string{
		"--old-bindir=" + oldBinDir,
		"--new-bindir=" + newBinDir,
		"--old-datadir=" + oldDataDir,
		"--new-datadir=" + newDataDir,
		"--check",
	}

	fmt.Println("Checking cluster compatibility...")
	checkCmd := exec.Command(filepath.Join(newBinDir, "pg_upgrade"), checkArgs...)
	checkOutput, err := checkCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pg_upgrade compatibility check failed: %w, output: %s", err, string(checkOutput))
	}

	// Run the actual upgrade
	upgradeArgs := []string{
		"--old-bindir=" + oldBinDir,
		"--new-bindir=" + newBinDir,
		"--old-datadir=" + oldDataDir,
		"--new-datadir=" + newDataDir,
	}

	fmt.Println("Performing upgrade...")
	upgradeCmd := exec.Command(filepath.Join(newBinDir, "pg_upgrade"), upgradeArgs...)
	upgradeOutput, err := upgradeCmd.CombinedOutput()
	p.lastStdout = string(upgradeOutput)
	p.lastStderr = string(upgradeOutput)

	if err != nil {
		return fmt.Errorf("pg_upgrade failed: %w, output: %s", err, string(upgradeOutput))
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

// copyFile copies a single file
func (p *Postgres) copyFile(src, dst string) error {
	cmd := exec.Command("cp", "-a", src, dst)
	return cmd.Run()
}

// copyDir copies a directory recursively
func (p *Postgres) copyDir(src, dst string) error {
	cmd := exec.Command("cp", "-a", src, dst)
	return cmd.Run()
}

func (p *Postgres) Stop() error {
	if err := p.ensureBinDir(); err != nil {
		return fmt.Errorf("failed to resolve binary directory: %w", err)
	}

	if p.DataDir == "" {
		return fmt.Errorf("DataDir not specified")
	}

	if !p.IsRunning() {
		return fmt.Errorf("PostgreSQL is not running")
	}

	args := []string{
		"-D", p.DataDir,
		"-m", "smart",
	}

	cmd := exec.Command(filepath.Join(p.BinDir, "pg_ctl"), args...)

	output, err := cmd.CombinedOutput()
	p.lastStdout = string(output)
	p.lastStderr = string(output)

	if err != nil {
		return fmt.Errorf("failed to stop PostgreSQL: %w, output: %s", err, string(output))
	}

	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for PostgreSQL to stop")
		case <-ticker.C:
			if !p.IsRunning() {
				return nil
			}
		}
	}
}

func (p *Postgres) Start() error {
	if p.BinDir == "" {
		return fmt.Errorf("BinDir not specified")
	}

	if p.DataDir == "" {
		return fmt.Errorf("DataDir not specified")
	}

	if !p.Exists() {
		return fmt.Errorf("PostgreSQL data directory does not exist, run InitDB first")
	}

	if p.IsRunning() {
		return fmt.Errorf("PostgreSQL is already running")
	}

	args := []string{
		"-D", p.DataDir,
		"-l", filepath.Join(p.DataDir, "logfile"),
		"start",
	}

	cmd := exec.Command(filepath.Join(p.BinDir, "pg_ctl"), args...)

	output, err := cmd.CombinedOutput()
	p.lastStdout = string(output)
	p.lastStderr = string(output)

	if err != nil {
		return fmt.Errorf("failed to start PostgreSQL: %w, output: %s", err, string(output))
	}

	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for PostgreSQL to start")
		case <-ticker.C:
			if p.IsRunning() {
				return nil
			}
		}
	}
}

func (p *Postgres) GetStderr() string {
	return p.lastStderr
}

func (p *Postgres) GetStdout() string {
	return p.lastStdout
}

// InstallExtensions installs the specified PostgreSQL extensions
func (p *Postgres) InstallExtensions(extensions []string) error {
	if len(extensions) == 0 {
		return nil
	}

	// Extension mapping for special cases
	extensionMap := map[string]string{
		"pgvector":        "vector",
		"pgsodium":        "pgsodium",
		"pgjwt":           "pgjwt",
		"pgaudit":         "pgaudit",
		"pg_tle":          "pg_tle",
		"pg_stat_monitor": "pg_stat_monitor",
		"pg_repack":       "pg_repack",
		"pg_plan_filter":  "pg_plan_filter",
		"pg_net":          "pg_net",
		"pg_jsonschema":   "pg_jsonschema",
		"pg_hashids":      "pg_hashids",
		"pg_cron":         "pg_cron",
		"pg_safeupdate":   "safeupdate",
		"index_advisor":   "index_advisor",
		"wal2json":        "wal2json",
	}

	// Check if PostgreSQL is running by testing connectivity
	if !p.IsRunning() {
		return fmt.Errorf("PostgreSQL is not running")
	}

	for _, ext := range extensions {
		ext = strings.TrimSpace(ext)
		if ext == "" {
			continue
		}

		extName := extensionMap[ext]
		if extName == "" {
			extName = ext
		}

		if err := p.installSingleExtension(ext, extName); err != nil {
			return fmt.Errorf("failed to install extension %s: %w", ext, err)
		}
	}

	return nil
}

// installSingleExtension installs a single PostgreSQL extension with special handling
func (p *Postgres) installSingleExtension(originalName, extensionName string) error {
	psqlPath := filepath.Join(p.BinDir, "psql")
	dbName := "postgres"
	user := "postgres"
	host := "localhost"
	port := 5432

	// Use config values if available
	if p.Config != nil && p.Config.Port != 0 {
		port = p.Config.Port
	}

	// For localhost, generally no password needed with trust auth
	// No SuperuserPassword field available in PostgresConf
	env := os.Environ()

	switch originalName {
	case "pg_cron":
		// Install pg_cron with special permissions
		cmd := exec.Command(psqlPath, "-h", host, "-p", strconv.Itoa(port), "-U", user, "-d", dbName, "-c",
			"CREATE EXTENSION IF NOT EXISTS pg_cron CASCADE;")
		cmd.Env = env

		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create pg_cron extension: %w, output: %s", err, string(output))
		}

		// Grant usage on cron schema
		cmd = exec.Command(psqlPath, "-h", host, "-p", strconv.Itoa(port), "-U", user, "-d", dbName, "-c",
			"GRANT USAGE ON SCHEMA cron TO postgres;")
		cmd.Env = env

		if _, err := cmd.CombinedOutput(); err != nil {
			// Non-fatal error for permission grant
			fmt.Printf("Warning: Failed to grant cron schema usage: %v\n", err)
		}

	case "pgsodium":
		// Install pgsodium and create initial key
		cmd := exec.Command(psqlPath, "-h", host, "-p", strconv.Itoa(port), "-U", user, "-d", dbName, "-c",
			"CREATE EXTENSION IF NOT EXISTS pgsodium CASCADE;")
		cmd.Env = env

		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create pgsodium extension: %w, output: %s", err, string(output))
		}

		// Create pgsodium key
		cmd = exec.Command(psqlPath, "-h", host, "-p", strconv.Itoa(port), "-U", user, "-d", dbName, "-c",
			"SELECT pgsodium.create_key();")
		cmd.Env = env

		if _, err := cmd.CombinedOutput(); err != nil {
			// Non-fatal error for key creation
			fmt.Printf("Warning: Failed to create pgsodium key: %v\n", err)
		}

	default:
		// Standard extension installation
		cmd := exec.Command(psqlPath, "-h", host, "-p", strconv.Itoa(port), "-U", user, "-d", dbName, "-c",
			fmt.Sprintf("CREATE EXTENSION IF NOT EXISTS %s CASCADE;", extensionName))
		cmd.Env = env

		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create extension %s: %w, output: %s", extensionName, err, string(output))
		}
	}

	return nil
}

// ListInstalledExtensions returns a list of installed PostgreSQL extensions
func (p *Postgres) ListInstalledExtensions() ([]ExtensionInfo, error) {
	results, err := p.SQL("SELECT extname, extversion FROM pg_extension WHERE extname NOT IN ('plpgsql') ORDER BY extname;")
	if err != nil {
		return nil, fmt.Errorf("failed to list installed extensions: %w", err)
	}

	var extensions []ExtensionInfo
	for _, row := range results {
		if nameVal, ok := row["extname"]; ok {
			if versionVal, ok := row["extversion"]; ok {
				name := fmt.Sprintf("%v", nameVal)
				version := fmt.Sprintf("%v", versionVal)
				extensions = append(extensions, ExtensionInfo{
					Name:    name,
					Version: version,
				})
			}
		}
	}

	return extensions, nil
}

// ListAvailableExtensions returns a list of available PostgreSQL extensions
func (p *Postgres) ListAvailableExtensions() ([]ExtensionInfo, error) {
	results, err := p.SQL("SELECT name, default_version FROM pg_available_extensions ORDER BY name;")
	if err != nil {
		return nil, fmt.Errorf("failed to list available extensions: %w", err)
	}

	var extensions []ExtensionInfo
	for _, row := range results {
		if nameVal, ok := row["name"]; ok {
			if versionVal, ok := row["default_version"]; ok {
				name := fmt.Sprintf("%v", nameVal)
				version := fmt.Sprintf("%v", versionVal)
				extensions = append(extensions, ExtensionInfo{
					Name:      name,
					Version:   version,
					Available: true,
				})
			}
		}
	}

	return extensions, nil
}

// GetSupportedExtensions returns the list of well-known supported extensions
func (p *Postgres) GetSupportedExtensions() []string {
	return []string{
		"pgvector",        // Vector similarity search
		"pgsodium",        // Modern cryptography
		"pgjwt",           // JSON Web Token support
		"pgaudit",         // PostgreSQL audit logging
		"pg_tle",          // Trusted Language Extensions
		"pg_stat_monitor", // Query performance monitoring
		"pg_repack",       // Table reorganization
		"pg_plan_filter",  // Query plan filtering
		"pg_net",          // Async networking
		"pg_jsonschema",   // JSON schema validation
		"pg_hashids",      // Short unique ID generation
		"pg_cron",         // Job scheduler
		"pg_safeupdate",   // Require WHERE clause in DELETE/UPDATE
		"index_advisor",   // Index recommendations
		"wal2json",        // WAL to JSON converter
	}
}

// Validate validates a PostgreSQL configuration file using the postgres binary
func (p *Postgres) Validate(config []byte) error {
	// Create a temporary file for the config
	tempFile, err := ioutil.TempFile("", "postgresql_validate_*.conf")
	if err != nil {
		return fmt.Errorf("failed to create temp config file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	// Write config to temp file
	if _, err := tempFile.Write(config); err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to write config to temp file: %w", err)
	}
	tempFile.Close()

	// Determine postgres binary path
	postgresBin := "postgres"
	if p.BinDir != "" {
		postgresBin = filepath.Join(p.BinDir, "postgres")
	}

	// Create a minimal temp data directory for validation
	tempDataDir, err := ioutil.TempDir("", "pg_validate_*")
	if err != nil {
		return fmt.Errorf("failed to create temp data directory: %w", err)
	}
	defer os.RemoveAll(tempDataDir)

	// Use postgres binary to validate the configuration
	// The -C flag reads a configuration parameter and exits
	cmd := exec.Command(postgresBin, "--config-file="+tempFile.Name(), "-D", tempDataDir, "-C", "data_directory")
	output, err := cmd.CombinedOutput()

	if err != nil {
		return p.parsePostgresValidationError(string(output), err)
	}

	return nil
}

// parsePostgresValidationError parses PostgreSQL validation errors and returns structured error
func (p *Postgres) parsePostgresValidationError(output string, originalErr error) error {
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse different error patterns
		if strings.Contains(line, "unrecognized configuration parameter") {
			// Example: LOG:  unrecognized configuration parameter "invalid_param"
			re := regexp.MustCompile(`unrecognized configuration parameter "([^"]+)"`)
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				return &pkg.ValidationError{
					Parameter: matches[1],
					Message:   "unrecognized configuration parameter",
					Raw:       line,
				}
			}
		}

		if strings.Contains(line, "invalid value for parameter") {
			// Example: FATAL:  invalid value for parameter "max_connections": "invalid"
			re := regexp.MustCompile(`invalid value for parameter "([^"]+)": "([^"]+)"`)
			if matches := re.FindStringSubmatch(line); len(matches) > 2 {
				return &pkg.ValidationError{
					Parameter: matches[1],
					Message:   fmt.Sprintf("invalid value: %s", matches[2]),
					Raw:       line,
				}
			}
		}

		if strings.Contains(line, "configuration file contains errors") {
			return &pkg.ValidationError{
				Message: "configuration file contains errors",
				Raw:     line,
			}
		}

		// Check for parameter out of range errors
		if strings.Contains(line, "out of range") {
			// Example: FATAL:  -1 is outside the valid range for parameter "max_connections" (1 .. 262143)
			re := regexp.MustCompile(`([0-9-]+) is outside the valid range for parameter "([^"]+)" \(([^)]+)\)`)
			if matches := re.FindStringSubmatch(line); len(matches) > 3 {
				return &pkg.ValidationError{
					Parameter: matches[2],
					Message:   fmt.Sprintf("value %s is outside valid range (%s)", matches[1], matches[3]),
					Raw:       line,
				}
			}
		}
	}

	// If we couldn't parse a specific error, return the original
	return &pkg.ValidationError{
		Message: fmt.Sprintf("configuration validation failed: %s", strings.TrimSpace(output)),
		Raw:     output,
	}
}

// stopTempPostgreSQL stops the temporary PostgreSQL instance
func (p *Postgres) stopTempPostgreSQL() error {
	stopArgs := []string{
		"-D", p.DataDir,
		"stop",
	}

	stopCmd := exec.Command(filepath.Join(p.BinDir, "pg_ctl"), stopArgs...)
	stopOutput, err := stopCmd.CombinedOutput()
	p.lastStdout = string(stopOutput)
	p.lastStderr = string(stopOutput)

	if err != nil {
		return fmt.Errorf("failed to stop PostgreSQL: %w, output: %s", err, string(stopOutput))
	}

	return nil
}