package installer

import (
	"context"
	_ "embed"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/flanksource/commons/deps"
	"github.com/go-task/task/v3"
	"github.com/go-task/task/v3/taskfile/ast"
)

//go:embed Taskfile.binaries.yaml
var taskfileBinaries []byte

// BinaryInfo contains metadata for a binary that can be installed
type BinaryInfo struct {
	Name           string
	TaskName       string
	DefaultVersion string
	VersionVar     string
}

// BinaryRegistry contains metadata for all supported binaries
var BinaryRegistry = map[string]BinaryInfo{
	"postgrest": {
		Name:           "postgrest",
		TaskName:       "install-postgrest",
		DefaultVersion: "12.2.3",
		VersionVar:     "POSTGREST_VERSION",
	},
	"wal-g": {
		Name:           "wal-g",
		TaskName:       "install-walg",
		DefaultVersion: "3.0.5",
		VersionVar:     "WALG_VERSION",
	},
	"postgres": {
		Name:           "postgres",
		TaskName:       "", // Uses commons deps InstallPostgres function
		DefaultVersion: "16.1.0",
		VersionVar:     "POSTGRES_VERSION",
	},
}

// Installer handles binary installation using embedded Taskfile
type Installer struct {
	tempDir string
}

// New creates a new installer instance
func New() *Installer {
	return &Installer{}
}

// CheckBinaryExists checks if a binary exists in PATH
func (i *Installer) CheckBinaryExists(binaryName string) bool {
	_, err := exec.LookPath(binaryName)
	return err == nil
}

// InstallBinary installs a binary using the registry metadata
// version: specific version to install (empty = use default from registry)
// targetDir: directory to install to (empty = use default and check PATH)
func (i *Installer) InstallBinary(binaryName, version, targetDir string) error {
	info, exists := BinaryRegistry[binaryName]
	if !exists {
		return fmt.Errorf("unknown binary: %s", binaryName)
	}

	// Use provided version or fall back to registry default
	if version == "" {
		version = info.DefaultVersion
	}

	// Skip PATH check if targetDir is specified (force install)
	if targetDir == "" && i.CheckBinaryExists(binaryName) {
		return nil // Already installed in PATH
	}

	// Handle postgres specially - use commons deps
	if binaryName == "postgres" {
		if targetDir == "" {
			targetDir = "/usr/local/bin"
		}
		return deps.InstallPostgres(version, targetDir)
	}

	// Handle other commons deps binaries
	if i.IsAvailableInCommons(binaryName) {
		if targetDir == "" {
			targetDir = "/usr/local/bin"
		}
		return deps.InstallDependency(binaryName, version, targetDir)
	}

	// Use local Taskfile for other binaries
	vars := map[string]string{
		info.VersionVar: version,
	}
	if targetDir != "" {
		vars["TARGET_BIN"] = targetDir
	}

	return i.runTask(info.TaskName, vars)
}

// runTask executes a specific task from the embedded Taskfile
func (i *Installer) runTask(taskName string, vars map[string]string) error {
	// Create temporary directory for Taskfile
	tempDir, err := ioutil.TempDir("", "postgres-installer-")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	i.tempDir = tempDir
	defer i.cleanup()

	// Write embedded Taskfile to temp directory
	taskfilePath := filepath.Join(tempDir, "Taskfile.yaml")
	if err := ioutil.WriteFile(taskfilePath, taskfileBinaries, 0644); err != nil {
		return fmt.Errorf("failed to write Taskfile: %w", err)
	}

	// Create Task executor with proper error handling
	executor := task.NewExecutor(
		task.WithDir(tempDir),
		task.WithEntrypoint(taskfilePath),
		task.WithColor(false), // Disable colors for now
		task.WithVerbose(true), // Enable verbose for debugging
	)

	// Setup the executor by reading the Taskfile
	if err := executor.Setup(); err != nil {
		return fmt.Errorf("failed to setup task executor: %w", err)
	}

	// Prepare variables for task execution
	taskVars := &ast.Vars{}
	for key, value := range vars {
		taskVars.Set(key, ast.Var{Value: value})
	}

	// Create call to execute the task
	call := &task.Call{
		Task: taskName,
		Vars: taskVars,
	}

	// Execute the task with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := executor.Run(ctx, call); err != nil {
		return fmt.Errorf("failed to execute task %s: %w", taskName, err)
	}

	return nil
}

// cleanup removes the temporary directory
func (i *Installer) cleanup() {
	if i.tempDir != "" {
		os.RemoveAll(i.tempDir)
		i.tempDir = ""
	}
}

// IsAvailableInCommons checks if a binary is available in flanksource/commons deps
func (i *Installer) IsAvailableInCommons(binaryName string) bool {
	// List of binaries available in commons deps that we want to support
	commonsBinaries := []string{"postgrest", "wal-g"} // postgres is handled specially
	for _, binary := range commonsBinaries {
		if binary == binaryName {
			return true
		}
	}
	return false
}

// IsBinarySupported checks if a binary is registered in the system
func (i *Installer) IsBinarySupported(binaryName string) bool {
	_, exists := BinaryRegistry[binaryName]
	return exists || i.IsAvailableInCommons(binaryName)
}

// GetDefaultVersion returns the default version for a binary
func (i *Installer) GetDefaultVersion(binaryName string) string {
	if info, exists := BinaryRegistry[binaryName]; exists {
		return info.DefaultVersion
	}
	
	// Default versions for commons deps binaries
	commonVersions := map[string]string{
		"postgrest": "v13.0.5", // From commons deps
		"wal-g":     "v3.0.5",   // From commons deps
	}
	
	if version, exists := commonVersions[binaryName]; exists {
		return version
	}
	
	return ""
}

// ListSupportedBinaries returns a list of all supported binaries
func (i *Installer) ListSupportedBinaries() []string {
	binarySet := make(map[string]bool)
	
	// Add binaries from local registry
	for name := range BinaryRegistry {
		binarySet[name] = true
	}
	
	// Add commons binaries (avoiding duplicates)
	commonsBinaries := []string{"postgrest", "wal-g"} 
	for _, name := range commonsBinaries {
		binarySet[name] = true
	}
	
	// Convert to slice
	binaries := make([]string, 0, len(binarySet))
	for name := range binarySet {
		binaries = append(binaries, name)
	}
	
	return binaries
}

// GetBinaryVersion gets the installed version of a binary
func (i *Installer) GetBinaryVersion(binaryName string) (string, error) {
	if !i.CheckBinaryExists(binaryName) {
		return "", fmt.Errorf("%s binary not found", binaryName)
	}
	
	cmd := exec.Command(binaryName, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get %s version: %w", binaryName, err)
	}
	
	return string(output), nil
}

// DEPRECATED METHODS - use new generic methods instead

// InstallPostgREST - DEPRECATED: use InstallBinary instead
func (i *Installer) InstallPostgREST(version string) error {
	return i.InstallBinary("postgrest", version, "")
}

// InstallWalG - DEPRECATED: use InstallBinary instead
func (i *Installer) InstallWalG(version string) error {
	return i.InstallBinary("wal-g", version, "")
}

// IsPostgRESTInstalled - DEPRECATED: use CheckBinaryExists instead
func (i *Installer) IsPostgRESTInstalled() bool {
	return i.CheckBinaryExists("postgrest")
}

// IsWalGInstalled - DEPRECATED: use CheckBinaryExists instead
func (i *Installer) IsWalGInstalled() bool {
	return i.CheckBinaryExists("wal-g")
}

// GetPostgRESTVersion - DEPRECATED: use GetBinaryVersion instead
func (i *Installer) GetPostgRESTVersion() (string, error) {
	return i.GetBinaryVersion("postgrest")
}

// GetWalGVersion - DEPRECATED: use GetBinaryVersion instead
func (i *Installer) GetWalGVersion() (string, error) {
	return i.GetBinaryVersion("wal-g")
}