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
	Host       string
	Port       int
	Username   string
	Password   utils.SensitiveString
	Database   string
	DryRun     bool
}

func (p *Postgres) WithoutAuth() *Postgres {
	return &Postgres{
		Config:   p.Config,
		DataDir:  p.DataDir,
		BinDir:   p.BinDir,
		Locale:   p.Locale,
		Encoding: p.Encoding,
		Database: p.Database,
	}
}

// Use types from pkg package
type ExtensionInfo = pkg.ExtensionInfo
type ValidationError = pkg.ValidationError

func NewRemotePostgres(host string, port int, username, password, database string) *Postgres {
	return &Postgres{
		Host:     host,
		Port:     port,
		Username: username,
		Password: utils.NewSensitiveString(password),
		Database: database,
	}
}

// NewPostgres creates a new PostgreSQL service instance
func NewPostgres(config *pkg.PostgresConf, dataDir string) *Postgres {
	return &Postgres{
		Config:   config,
		Database: "postgres",
		DataDir:  dataDir,
		Locale:   "C",
		Encoding: "UTF8",
	}
}

func (p *Postgres) IsRemote() bool {
	return p.Host != "" && p.Port != 0 && p.DataDir == ""
}

func (p *Postgres) Validate() error {
	if !p.Password.IsEmpty() {
		clicky.RedactSecretValues(p.Password.Value())
	}

	// Try to auto-detect directories
	detectedDirs, err := utils.DetectPostgreSQLDirs()
	if err != nil {
		return err
	}

	// Apply overrides from flags or use detected values
	if p.BinDir == "" && detectedDirs != nil {
		p.BinDir = detectedDirs.BinDir
	}
	if p.DataDir == "" && detectedDirs != nil {
		p.DataDir = detectedDirs.DataDir
	}

	if p.DataDir == "" {
		return fmt.Errorf("data directory not detected, use --data-dir flag")
	}
	if p.BinDir == "" {
		return fmt.Errorf("postgres binary directory not set")
	}

	return utils.CheckPGDATAPermissions(p.DataDir)
}

// Note: For embedded postgres functionality, use pkg/embedded.NewEmbeddedPostgres() instead

// DescribeConfig executes `postgres --describe-config` and returns parsed parameters
func (p *Postgres) DescribeConfig() ([]schemas.Param, error) {
	if err := p.Validate(); err != nil {
		return nil, err
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
	if err := p.Validate(); err != nil {
		return 0, err
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

func (p *Postgres) Ping() error {
	_, err := p.SQL("SELECT 1")
	return err
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

// GetFullVersion returns the full PostgreSQL version string
// Example: "PostgreSQL 17.5 (Debian 17.5-1.pgdg120+1) on x86_64-pc-linux-gnu, compiled by..."
func (p *Postgres) GetFullVersion() string {
	if err := p.ensureBinDir(); err != nil {
		return ""
	}

	process := clicky.Exec(filepath.Join(p.BinDir, "postgres"), "--version").Run()
	if process.Err != nil {
		return ""
	}

	return strings.TrimSpace(process.GetStdout())
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

func (p *Postgres) GetConnectionString() string {
	connStr := "sslmode=disable"

	if p.Database != "" {
		connStr += " dbname=" + p.Database
	}

	if p.Host != "" {
		connStr += " host=" + p.Host
	}

	if p.Port != 0 {
		connStr += " port=" + strconv.Itoa(p.Port)
	}

	if p.Username != "" {
		connStr += " user=" + p.Username
	}

	if p.Password != "" {
		connStr += " password=" + p.Password.Value()
	}
	return connStr
}

func (p *Postgres) GetConnection() (*sql.DB, error) {
	db, err := sql.Open("postgres", p.GetConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	return db, nil
}

func (p *Postgres) WithConnection(fn func(db *sql.DB) error) error {
	var tempDB *Postgres
	if !p.IsRemote() && !p.IsRunning() {
		tempDB = p.WithoutAuth()
		tempDB.Start()
		defer tempDB.Stop()
	}
	db, err := p.GetConnection()
	if err != nil {
		return err
	}
	defer db.Close()

	return fn(db)
}

func (p *Postgres) SQL(sqlQuery string, args ...any) ([]map[string]interface{}, error) {
	results := []map[string]interface{}{}

	if err := p.WithConnection(func(db *sql.DB) error {

		clicky.SQL(sqlQuery)
		rows, err := db.Query(sqlQuery, args...)
		if err != nil {
			return err
		}
		defer rows.Close()

		columns, err := rows.Columns()
		if err != nil {
			return fmt.Errorf("failed to get columns: %w", err)
		}

		for rows.Next() {
			values := make([]interface{}, len(columns))
			valuePtrs := make([]interface{}, len(columns))

			for i := range values {
				valuePtrs[i] = &values[i]
			}

			if err := rows.Scan(valuePtrs...); err != nil {
				return fmt.Errorf("failed to scan row: %w", err)
			}

			row := make(map[string]interface{})
			for i, column := range columns {
				row[column] = values[i]
			}

			results = append(results, row)
		}

		if err := rows.Err(); err != nil {
			return fmt.Errorf("error iterating rows: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
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

	if p.DryRun {
		clicky.Infof("Dry run enabled, skipping backup execution. Command: pg_dump %s", strings.Join(args, " "))
		return nil
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

	return cmd
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

	fmt.Printf("🛠️  Ensuring database '%s' exists...\n", name)
	if p.DryRun {
		clicky.Infof("[DRYRUN] skipping database creation for '%s'", name)
		return nil
	}
	_, err := p.SQL("CREATE DATABASE ?", name)
	return err
}

type InitDBOptions struct {
	Username   string
	Password   utils.SensitiveString
	InitDBArgs string
	WALDir     string
	AuthMethod string
}

func (p *Postgres) InitDB() error {
	return p.InitDBWithOptions(InitDBOptions{})
}

func (p *Postgres) InitDBWithOptions(opts InitDBOptions) error {
	if p.BinDir == "" {
		return fmt.Errorf("BinDir not specified")
	}

	if p.DataDir == "" {
		return fmt.Errorf("DataDir not specified")
	}

	if err := os.MkdirAll(p.DataDir, 0700); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	if opts.Username == "" {
		opts.Username = "postgres"
	}

	if opts.AuthMethod == "" {
		opts.AuthMethod = "trust"
	}

	args := []string{
		"-D", p.DataDir,
		"-A", opts.AuthMethod,
		"-U", opts.Username,
	}

	if opts.WALDir != "" {
		if err := os.MkdirAll(opts.WALDir, 0700); err != nil {
			return fmt.Errorf("failed to create WAL directory: %w", err)
		}
		args = append(args, "--waldir="+opts.WALDir)
	}

	var passwordFile string
	if !opts.Password.IsEmpty() {
		tmpFile, err := os.CreateTemp("", "pg_password_*")
		if err != nil {
			return fmt.Errorf("failed to create password temp file: %w", err)
		}
		passwordFile = tmpFile.Name()
		defer os.Remove(passwordFile)

		if _, err := tmpFile.WriteString(string(opts.Password) + "\n"); err != nil {
			tmpFile.Close()
			return fmt.Errorf("failed to write password file: %w", err)
		}
		tmpFile.Close()

		if err := os.Chmod(passwordFile, 0600); err != nil {
			return fmt.Errorf("failed to set password file permissions: %w", err)
		}

		args = append(args, "--pwfile="+passwordFile)
	}

	if opts.InitDBArgs != "" {
		additionalArgs := strings.Fields(opts.InitDBArgs)
		args = append(args, additionalArgs...)
	}

	if p.DryRun {
		clicky.Infof("[DRYRUN] initdb %s", strings.Join(args, " "))
		return nil
	}

	clicky.Infof("🔧 Initializing PostgreSQL database...")

	process := clicky.Exec(filepath.Join(p.BinDir, "initdb"), args...).Run()

	p.lastStdout = process.GetStdout()
	p.lastStderr = process.GetStderr()

	if process.Err != nil {
		return fmt.Errorf("initdb failed: %w, output: %s", process.Err, process.Out())
	}

	clicky.Infof("✅ PostgreSQL database initialized successfully")

	return nil
}

// ResetPassword resets the PostgreSQL superuser password using a temporary server instance
func (p *Postgres) ResetPassword(newPassword utils.SensitiveString) error {
	if newPassword.IsEmpty() {
		return fmt.Errorf("new password not specified")
	}

	if p.Username == "" {
		return fmt.Errorf("PGUSER not specified")
	}

	if p.DryRun {
		clicky.Infof("[DRYRUN] skipping password reset for user %s", p.Username)
		return nil
	}
	clicky.Infof("Resetting password for user %s", p.Username)

	psqlProcess, err := p.SQL(fmt.Sprintf("ALTER USER %s PASSWORD '%s'", p.Username, newPassword.Value()))
	if err != nil {
		return err
	}
	clicky.MustPrint(psqlProcess)

	clicky.Infof("✅ Password reset process completed")
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

	if p.DryRun {
		clicky.Infof("[DRYRUN] initdb %s", strings.Join(args, " "))
		return nil
	}

	process := clicky.Exec(filepath.Join(binDir, "initdb"), args...).Run()
	p.lastStdout = process.GetStdout()
	p.lastStderr = process.GetStderr()

	if process.Err != nil {
		return fmt.Errorf("initdb failed: %w, output: %s", process.Err, process.Out())
	}

	return nil
}

// GetCurrentConf queries the running PostgreSQL instance for current configuration
func (p *Postgres) GetCurrentConf() (config.ConfSettings, error) {
	if err := p.ensureBinDir(); err != nil {
		return nil, fmt.Errorf("failed to resolve binary directory: %w", err)
	}

	if !p.IsRunning() {
		return nil, fmt.Errorf("PostgreSQL is not running, cannot query current configuration")
	}

	results, err := p.SQL("SELECT * FROM pg_settings where source != 'default'")
	if err != nil {
		return nil, fmt.Errorf("failed to execute pg_settings query: %w", err)
	}

	return config.LoadSettingsFromQuery(results)
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
	if p.DryRun {
		clicky.Infof("[DRYRUN] stopping Postgres %s", p.DataDir)
		return nil
	}

	clicky.Infof(clicky.Text("").Add(icons.Stop).Appendf("Stopping Postgres.. %s", p.DataDir).Styles("text-orange-500").ANSI())

	res := p.Pg_ctl("stop", "-m", "smart").Run().Result()
	if res.Error != nil {
		return fmt.Errorf("failed to stop PostgreSQL: %s", res.Pretty().ANSI())
	}

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

	if p.DryRun {
		clicky.Infof("[DRYRUN] starting Postgres %s", p.DataDir)
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
func (p *Postgres) ValidateConfig(config []byte) error {
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
