package pkg

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// WalG represents a WAL-G backup service instance
type WalG struct {
	Config *WalgConf
}

// NewWalG creates a new WAL-G service instance
func NewWalG(config *WalgConf) *WalG {
	return &WalG{
		Config: config,
	}
}

// Health performs a comprehensive health check of the WAL-G service
func (w *WalG) Health() error {
	if w == nil {
		return fmt.Errorf("WAL-G service is nil")
	}
	if w.Config == nil {
		return fmt.Errorf("WAL-G configuration not provided")
	}

	if !w.Config.Enabled {
		return fmt.Errorf("WAL-G is disabled")
	}

	// Check if storage configuration is present
	if (w.Config.S3Prefix == nil || *w.Config.S3Prefix == "") &&
		(w.Config.GsPrefix == nil || *w.Config.GsPrefix == "") &&
		(w.Config.AzPrefix == nil || *w.Config.AzPrefix == "") &&
		(w.Config.FilePrefix == nil || *w.Config.FilePrefix == "") {
		return fmt.Errorf("WAL-G storage configuration required (S3Prefix, GsPrefix, AzPrefix, or FilePrefix)")
	}

	// Check if WAL-G binary is available
	_, err := exec.LookPath("wal-g")
	if err != nil {
		return fmt.Errorf("wal-g binary not found in PATH: %w", err)
	}

	// Set up environment variables for WAL-G command
	env := w.buildEnvironment()

	// Test WAL-G connectivity by listing backups
	cmd := exec.Command("wal-g", "backup-list", "--json")
	cmd.Env = env

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("WAL-G backup-list failed: %w, output: %s", err, string(output))
	}

	// Try to parse the JSON output to ensure it's valid
	var backupList interface{}
	if len(output) > 0 {
		if err := json.Unmarshal(output, &backupList); err != nil {
			return fmt.Errorf("WAL-G returned invalid JSON: %w", err)
		}
	}

	return nil
}

// BackupPush creates a new backup using WAL-G
func (w *WalG) BackupPush() error {
	if w == nil {
		return fmt.Errorf("WAL-G service is nil")
	}
	if w.Config == nil {
		return fmt.Errorf("WAL-G configuration not provided")
	}

	if !w.Config.Enabled {
		return fmt.Errorf("WAL-G is disabled")
	}

	dataDir := w.Config.PostgresqlDataDir
	if dataDir == "" {
		return fmt.Errorf("PostgreSQL data directory not specified")
	}

	env := w.buildEnvironment()
	cmd := exec.Command("wal-g", "backup-push", dataDir)
	cmd.Env = env

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("WAL-G backup-push failed: %w, output: %s", err, string(output))
	}

	return nil
}

// BackupList retrieves the list of available backups
func (w *WalG) BackupList() ([]BackupInfo, error) {
	if w == nil {
		return nil, fmt.Errorf("WAL-G service is nil")
	}
	if w.Config == nil {
		return nil, fmt.Errorf("WAL-G configuration not provided")
	}

	if !w.Config.Enabled {
		return nil, fmt.Errorf("WAL-G is disabled")
	}

	env := w.buildEnvironment()
	cmd := exec.Command("wal-g", "backup-list", "--json")
	cmd.Env = env

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("WAL-G backup-list failed: %w, output: %s", err, string(output))
	}

	if len(output) == 0 {
		return []BackupInfo{}, nil
	}

	var backups []BackupInfo
	if err := json.Unmarshal(output, &backups); err != nil {
		return nil, fmt.Errorf("failed to parse backup list JSON: %w", err)
	}

	return backups, nil
}

// BackupFetch restores a backup by name
func (w *WalG) BackupFetch(backupName, targetDir string) error {
	if w == nil {
		return fmt.Errorf("WAL-G service is nil")
	}
	if w.Config == nil {
		return fmt.Errorf("WAL-G configuration not provided")
	}

	if !w.Config.Enabled {
		return fmt.Errorf("WAL-G is disabled")
	}

	if backupName == "" {
		return fmt.Errorf("backup name is required")
	}

	if targetDir == "" {
		return fmt.Errorf("target directory is required")
	}

	env := w.buildEnvironment()
	cmd := exec.Command("wal-g", "backup-fetch", targetDir, backupName)
	cmd.Env = env

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("WAL-G backup-fetch failed: %w, output: %s", err, string(output))
	}

	return nil
}

// DeleteRetain deletes old backups, keeping the specified number
func (w *WalG) DeleteRetain(retainCount int) error {
	if w == nil {
		return fmt.Errorf("WAL-G service is nil")
	}
	if w.Config == nil {
		return fmt.Errorf("WAL-G configuration not provided")
	}

	if !w.Config.Enabled {
		return fmt.Errorf("WAL-G is disabled")
	}

	if retainCount <= 0 {
		return fmt.Errorf("retain count must be positive")
	}

	env := w.buildEnvironment()
	cmd := exec.Command("wal-g", "delete", "retain", fmt.Sprintf("%d", retainCount), "--confirm")
	cmd.Env = env

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("WAL-G delete retain failed: %w, output: %s", err, string(output))
	}

	return nil
}

// GetStatus returns detailed WAL-G service status
func (w *WalG) GetStatus() (*WalgStatus, error) {
	if w == nil {
		return nil, fmt.Errorf("WAL-G service is nil")
	}
	if w.Config == nil {
		return nil, fmt.Errorf("WAL-G configuration not provided")
	}

	status := &WalgStatus{
		Enabled:   w.Config.Enabled,
		CheckTime: time.Now(),
		Storage:   w.getStorageType(),
		Schedule:  w.Config.BackupSchedule,
		Retention: w.Config.BackupRetainCount,
	}

	if !w.Config.Enabled {
		status.Message = "WAL-G is disabled"
		return status, nil
	}

	// Perform health check
	if err := w.Health(); err != nil {
		status.Healthy = false
		status.Error = err.Error()
		return status, nil
	}

	status.Healthy = true

	// Get backup count
	backups, err := w.BackupList()
	if err != nil {
		status.Error = fmt.Sprintf("Failed to get backup list: %v", err)
		return status, nil
	}
	status.BackupCount = len(backups)

	if len(backups) > 0 {
		// Find the most recent backup
		var mostRecent *BackupInfo
		for i := range backups {
			if mostRecent == nil || backups[i].StartTime.After(mostRecent.StartTime) {
				mostRecent = &backups[i]
			}
		}
		if mostRecent != nil {
			status.LastBackup = &mostRecent.StartTime
		}
	}

	return status, nil
}

// Install installs WAL-G binary with optional version and target directory
func (w *WalG) Install(version, targetDir string) error {
	return nil
}

// IsInstalled checks if WAL-G is installed in PATH
func (w *WalG) IsInstalled() bool {
	return false
}

// InstalledVersion returns the installed WAL-G version
func (w *WalG) InstalledVersion() (string, error) {
	cmd := exec.Command("wal-g", "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get wal-g version: %w", err)
	}
	return string(output), nil
}

// buildEnvironment constructs the environment variables for WAL-G commands
func (w *WalG) buildEnvironment() []string {
	var env []string

	// PostgreSQL data directory
	if w.Config.PostgresqlDataDir != "" {
		env = append(env, "WALG_POSTGRESQL_DATA_DIR="+w.Config.PostgresqlDataDir)
	}

	// Storage configuration
	if w.Config.S3Prefix != nil && *w.Config.S3Prefix != "" {
		env = append(env, "WALG_S3_PREFIX="+*w.Config.S3Prefix)
	}
	if w.Config.GsPrefix != nil && *w.Config.GsPrefix != "" {
		env = append(env, "WALG_GS_PREFIX="+*w.Config.GsPrefix)
	}
	if w.Config.AzPrefix != nil && *w.Config.AzPrefix != "" {
		env = append(env, "WALG_AZ_PREFIX="+*w.Config.AzPrefix)
	}
	if w.Config.FilePrefix != nil && *w.Config.FilePrefix != "" {
		env = append(env, "WALG_FILE_PREFIX="+*w.Config.FilePrefix)
	}

	// Stream commands
	if w.Config.StreamCreateCommand != nil && *w.Config.StreamCreateCommand != "" {
		env = append(env, "WALG_STREAM_CREATE_COMMAND="+*w.Config.StreamCreateCommand)
	}
	if w.Config.StreamRestoreCommand != nil && *w.Config.StreamRestoreCommand != "" {
		env = append(env, "WALG_STREAM_RESTORE_COMMAND="+*w.Config.StreamRestoreCommand)
	}

	// S3 configuration
	if w.Config.S3Region != "" {
		env = append(env, "AWS_REGION="+w.Config.S3Region)
	}
	if w.Config.S3AccessKey != nil && *w.Config.S3AccessKey != "" {
		env = append(env, "AWS_ACCESS_KEY_ID="+*w.Config.S3AccessKey)
	}
	if w.Config.S3SecretKey != nil && *w.Config.S3SecretKey != "" {
		env = append(env, "AWS_SECRET_ACCESS_KEY="+*w.Config.S3SecretKey)
	}
	if w.Config.S3SessionToken != nil && *w.Config.S3SessionToken != "" {
		env = append(env, "AWS_SESSION_TOKEN="+*w.Config.S3SessionToken)
	}
	if w.Config.S3Endpoint != nil && *w.Config.S3Endpoint != "" {
		env = append(env, "AWS_ENDPOINT="+*w.Config.S3Endpoint)
	}
	if w.Config.S3UseSsl != nil && !*w.Config.S3UseSsl {
		env = append(env, "WALG_S3_SSL=false")
	}

	// Google Cloud configuration
	if w.Config.GsServiceAccountKey != nil && *w.Config.GsServiceAccountKey != "" {
		env = append(env, "GOOGLE_APPLICATION_CREDENTIALS="+*w.Config.GsServiceAccountKey)
	}

	// Azure configuration
	if w.Config.AzAccountName != nil && *w.Config.AzAccountName != "" {
		env = append(env, "AZURE_STORAGE_ACCOUNT="+*w.Config.AzAccountName)
	}
	if w.Config.AzAccountKey != nil && *w.Config.AzAccountKey != "" {
		env = append(env, "AZURE_STORAGE_KEY="+*w.Config.AzAccountKey)
	}
	// AzStorageSasToken field removed - not supported in current schema

	return env
}

// getStorageType determines which storage backend is configured
func (w *WalG) getStorageType() string {
	if w.Config.S3Prefix != nil && *w.Config.S3Prefix != "" {
		return "S3"
	}
	if w.Config.GsPrefix != nil && *w.Config.GsPrefix != "" {
		return "GCS"
	}
	if w.Config.AzPrefix != nil && *w.Config.AzPrefix != "" {
		return "Azure"
	}
	if w.Config.FilePrefix != nil && *w.Config.FilePrefix != "" {
		return "File"
	}
	return "None"
}

// BackupInfo represents information about a WAL-G backup
type BackupInfo struct {
	BackupName       string    `json:"backup_name"`
	StartTime        time.Time `json:"start_time"`
	FinishTime       time.Time `json:"finish_time"`
	UncompressedSize int64     `json:"uncompressed_size"`
	CompressedSize   int64     `json:"compressed_size"`
	DataSize         int64     `json:"data_size"`
	IsPermanent      bool      `json:"is_permanent"`
}

// WalgStatus represents the status of a WAL-G service
type WalgStatus struct {
	Enabled     bool       `json:"enabled"`
	Healthy     bool       `json:"healthy"`
	CheckTime   time.Time  `json:"check_time"`
	Storage     string     `json:"storage"`
	Schedule    string     `json:"schedule"`
	Retention   int        `json:"retention"`
	BackupCount int        `json:"backup_count"`
	LastBackup  *time.Time `json:"last_backup,omitempty"`
	Message     string     `json:"message,omitempty"`
	Error       string     `json:"error,omitempty"`
}
