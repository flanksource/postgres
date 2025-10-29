package server

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/api/icons"
	"github.com/flanksource/clicky/exec"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/commons/properties"
	_ "github.com/lib/pq"

	"github.com/flanksource/postgres/pkg"
	"github.com/flanksource/postgres/pkg/config"
	"github.com/flanksource/postgres/pkg/schemas"
	"github.com/flanksource/postgres/pkg/utils"
)

type Postgres struct {
	Config     *pkg.PostgresConf
	DataDir    string
	BinDir     string // Auto-resolved based on detected version
	Locale     string
	Encoding   string
	lastStdout string
	lastStderr string
	host       string
	port       int
	username   string
	password   utils.SensitiveString
	database   string
}

// Use types from pkg package
type ExtensionInfo = pkg.ExtensionInfo
type ValidationError = pkg.ValidationError

// NewPostgres creates a new PostgreSQL service instance
func NewPostgres(config *pkg.PostgresConf, dataDir string) *Postgres {
	return &Postgres{
		Config:   config,
		database: "postgres",
		DataDir:  dataDir,
		Locale:   "C",
		Encoding: "UTF8",
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
	process := clicky.Exec(postgresPath, "--describe-config").Run()
	if process.Err != nil {
		return nil, fmt.Errorf("failed to run postgres --describe-config: %w", process.Err)
	}

	return schemas.ParseDescribeConfig(process.GetStdout())
}

// GetControlData executes `pg_controldata` and returns parsed control data
func (p *Postgres) GetControlData() (*config.ControlData, error) {
	if p.BinDir == "" {
		return nil, fmt.Errorf("postgres binary directory not set")
	}

	if p.DataDir == "" {
		return nil, fmt.Errorf("data directory not specified")
	}

	process := p.bin("pg_controldata", "-D", p.DataDir).Run()
	if process.Err != nil {
		return nil, fmt.Errorf("failed to run pg_controldata: %w", process.Err)
	}

	return config.ParseControlData(process.GetStdout())
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

	process := clicky.Exec(
		filepath.Join(p.BinDir, "pg_isready"),
		"-h", host,
		"-p", strconv.Itoa(port),
	).Run()

	if process.Err != nil {
		return fmt.Errorf("pg_isready failed: %w", process.Err)
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

	process := clicky.Exec(filepath.Join(p.BinDir, "postgres"), "--version").Run()
	if process.Err != nil {
		return ""
	}

	versionStr := process.GetStdout()
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

	// Use config values if available
	if p.Config != nil {
		if p.Config.ListenAddresses != "" && p.Config.ListenAddresses != "*" {
			p.host = p.Config.ListenAddresses
		}
		if p.Config.Port != 0 {
			p.port = p.Config.Port
		}
		// No SuperuserPassword field available in PostgresConf
	}

	connStr := "sslmode=disable"

	if p.database != "" {
		connStr += " dbname=" + p.database
	}

	if p.host != "" {
		connStr += " host=" + p.host
	}

	if p.port != 0 {
		connStr += " port=" + strconv.Itoa(p.port)
	}

	if p.username != "" {
		connStr += " user=" + p.username
	}

	if !p.password.IsEmpty() {
		connStr += " password=" + p.password.Value()
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

	process := clicky.Exec(filepath.Join(p.BinDir, "pg_dump"), args...).Run()

	// Only set password for non-localhost connections or if explicitly provided
	// No SuperuserPassword field available in PostgresConf

	p.lastStdout = process.GetStdout()
	p.lastStderr = process.GetStderr()

	if process.Err != nil {
		return fmt.Errorf("backup failed: %w, output: %s", process.Err, process.Out())
	}

	return nil
}
func (p *Postgres) bin(name string, args ...string) *exec.Process {

	if p.DataDir == "" {
		panic("Data dir not specifed")
	}

	if err := p.ensureBinDir(); err != nil {
		panic(fmt.Errorf("failed to resolve binary directory: %w", err))
	}

	cmd := clicky.Exec(filepath.Join(p.BinDir, name), args...)
	cmd.Env = map[string]string{
		"PGDATA": p.DataDir,
	}
	cmd.Timeout = 10 * time.Second

	return cmd.Debug()
}

func WrappedError(err error) exec.WrapperFunc {
	return func(args ...any) (*exec.ExecResult, error) {
		return nil, err
	}
}
func (p *Postgres) CreateDatabase(name string) error {
	if name == "" {
		return fmt.Errorf("database name not specified")
	}

	res := p.bin("createdb", name).Run().Result()
	if res.Error != nil {
		return fmt.Errorf("failed to create database '%s': %w", name, res.Error)
	}
	fmt.Println(res.Pretty().ANSI())

	return nil
}

func (p *Postgres) InitDB() error {
	if p.BinDir == "" {
		return fmt.Errorf("BinDir not specified")
	}

	if p.DataDir == "" {
		return fmt.Errorf("DataDir not specified")
	}

	if err := os.MkdirAll(p.DataDir, 0700); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	args := []string{
		"-D", p.DataDir,
		"-A", "trust", // Use trust authentication for localhost by default
		// "--locale=" + p.Locale,
		// "--encoding=" + p.Encoding,
		"-U", "postgres", // Always use postgres superuser
	}

	process := clicky.Exec(filepath.Join(p.BinDir, "initdb"), args...).Run()

	// Generally no password needed for initdb with trust auth
	// No SuperuserPassword field available in PostgresConf

	p.lastStdout = process.GetStdout()
	p.lastStderr = process.GetStderr()

	if process.Err != nil {
		return fmt.Errorf("initdb failed: %w, output: %s", process.Err, process.Out())
	}

	return nil
}

// ResetPassword resets the PostgreSQL superuser password using a temporary server instance
func (p *Postgres) ResetPassword(newPassword utils.SensitiveString) error {
	if newPassword.IsEmpty() {
		return fmt.Errorf("new password not specified")
	}

	if !p.IsRunning() {
		p.Start()
		defer p.Stop()
	}
	resetSQL := fmt.Sprintf("ALTER USER %s PASSWORD '%s';", p.username, newPassword.Value())

	fmt.Printf("🔑 Resetting password for user %s...\n", p.username)

	psqlProcess, err := p.SQL(resetSQL)
	if err != nil {
		return err
	}
	clicky.MustPrint(psqlProcess)

	fmt.Println("✅ Password reset process completed")
	return nil
}

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
		fmt.Printf(clicky.Text("").Add(icons.ArrowUp).Append(" Upgrading Postgres from", "font-bold text-red-500").Append(version).Append("to").Append(nextVersion).String())

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

	clicky.Exec("cp", "-a", filepath.Join(p.DataDir, "main"), backupPath)

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
	initdbConf := oldConf.ToMap().ForInitDB()
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

// validateCluster validates a PostgreSQL cluster
func (p *Postgres) validateCluster(binDir, dataDir string, expectedVersion int) error {

	newCluster := &Postgres{
		DataDir: dataDir,
		BinDir:  binDir,
	}

	info, err := newCluster.Info()
	if err != nil {
		return fmt.Errorf("failed to get cluster info: %w", err)
	}

	fmt.Println(clicky.Text("📝 Cluster information:", "font-bold").Append(clicky.MustFormat(info)).Append("-----------------"))

	if info.VersionNumber != expectedVersion {
		return fmt.Errorf("expected PostgreSQL %d, but cluster reports version %d", expectedVersion, info.VersionNumber)
	}

	return nil
}

// initNewCluster initializes a new PostgreSQL cluster
func (p *Postgres) initNewCluster(binDir, dataDir string) error {
	if err := clicky.Exec("mkdir", "-p", dataDir).Run().Result().Error; err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	process := clicky.Exec(
		filepath.Join(binDir, "initdb"),
		"-D", dataDir,
		"--encoding="+p.Encoding,
		"--locale="+p.Locale,
	).Run()
	p.lastStdout = process.GetStdout()
	p.lastStderr = process.GetStderr()

	if process.Err != nil {
		return fmt.Errorf("initdb failed: %w, output: %s", process.Err, process.Out())
	}

	return nil
}

// initNewClusterWithConf initializes a new PostgreSQL cluster with specific configuration
func (p *Postgres) initNewClusterWithConf(binDir, dataDir string, conf config.Conf) error {
	if err := clicky.Exec("mkdir", "-p", dataDir).Run().Result().Error; err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Build initdb command
	args := []string{
		"-D", dataDir,
		"-U", "postgres",
		"-A", "trust",
	}

	// Add initdb-specific arguments from configuration
	args = append(args, conf.AsInitDBArgs()...)

	process := clicky.Exec(filepath.Join(binDir, "initdb"), args...).Run()
	p.lastStdout = process.GetStdout()
	p.lastStderr = process.GetStderr()

	if process.Err != nil {
		return fmt.Errorf("initdb failed: %w, output: %s", process.Err, process.Out())
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

// GetCurrentConf queries the running PostgreSQL instance for current configuration
func (p *Postgres) GetCurrentConf() (config.ConfSettings, error) {
	if err := p.ensureBinDir(); err != nil {
		return nil, fmt.Errorf("failed to resolve binary directory: %w", err)
	}

	if !p.IsRunning() {
		return nil, fmt.Errorf("PostgreSQL is not running, cannot query current configuration")
	}

	results, err := p.SQL("SELECT * FROM pg_settings WHERE setting IS DISTINCT FROM boot_val")
	if err != nil {
		return nil, fmt.Errorf("failed to execute pg_settings query: %w", err)
	}

	return config.LoadSettingsFromQuery(results)
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
	process := clicky.Exec("cp", "-a", src, dst).Run()
	return process.Err
}

// copyDir copies a directory recursively
func (p *Postgres) copyDir(src, dst string) error {
	process := clicky.Exec("cp", "-a", src, dst).Run()
	return process.Err
}

func (p *Postgres) Pg_ctl(args ...string) *exec.Process {
	if err := p.ensureBinDir(); err != nil {
		panic(fmt.Errorf("failed to resolve binary directory: %w", err))
	}

	if p.DataDir == "" {
		panic("DataDir not specified")

	}
	cmd := clicky.Exec(filepath.Join(p.BinDir, "pg_ctl"), append([]string{"-D", p.DataDir}, args...)...)
	return cmd.Debug()
}

func (p *Postgres) Stop() error {

	if !p.IsRunning() {
		logger.Warnf("Postgres is not running")
		return nil
	}

	fmt.Println(clicky.Text("").Add(icons.Stop).Appendf("Stopping Postgres.. %s", p.DataDir).Styles("text-orange-500").ANSI())

	res := p.Pg_ctl("stop", "-m", "smart").Run().Result()
	if res.Error != nil {
		return fmt.Errorf("failed to stop PostgreSQL: %w", res.Error)
	}

	fmt.Println(res.Pretty().ANSI())

	timeout := properties.Duration(30*time.Second, "stop.timeout")
	startTime := time.Now()
	for {
		if !p.IsRunning() {
			break
		}
		if time.Since(startTime) > timeout {
			return fmt.Errorf("timeout waiting for PostgreSQL to stop")
		}
		time.Sleep(1 * time.Second)
	}

	if p.IsRunning() {
		return fmt.Errorf("PostgreSQL did not stop successfully")
	}

	return nil
}

func (p *Postgres) Start() error {
	if p.IsRunning() {
		logger.Warnf("Postgres is already running")
		return nil
	}

	fmt.Println(clicky.Text("").Add(icons.Start).Appendf("Starting Postgres.. %s", p.DataDir).Styles("text-green-500").ANSI())

	res := p.Pg_ctl("start").WithTimeout(5 * time.Second).Run().Result()
	if res.Error != nil {
		fmt.Println("failed to start PostgreSQL: %w, continuing anyway", res.Error)
	}
	fmt.Println(res.Pretty().ANSI())

	timeout := properties.Duration(300*time.Second, "start.timeout")
	startTime := time.Now()
	for {
		if p.IsRunning() {
			break
		}
		if time.Since(startTime) > timeout {
			return fmt.Errorf("timeout waiting for PostgreSQL to start")
		}
		time.Sleep(1 * time.Second)
	}

	if !p.IsRunning() {
		return fmt.Errorf("PostgreSQL did not start successfully")
	}
	return nil

}

func (p *Postgres) GetStderr() string {
	return p.lastStderr
}

func (p *Postgres) GetStdout() string {
	return p.lastStdout
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
	process := clicky.Exec(postgresBin, "--config-file="+tempFile.Name(), "-D", tempDataDir, "-C", "data_directory").Run()

	if process.Err != nil {
		return p.parsePostgresValidationError(process.Out(), process.Err)
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

	stopProcess := clicky.Exec(filepath.Join(p.BinDir, "pg_ctl"), stopArgs...).Run()
	p.lastStdout = stopProcess.GetStdout()
	p.lastStderr = stopProcess.GetStderr()

	if stopProcess.Err != nil {
		return fmt.Errorf("failed to stop PostgreSQL: %w, output: %s", stopProcess.Err, stopProcess.Out())
	}

	return nil
}
