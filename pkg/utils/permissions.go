package utils

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"syscall"
)

// PermissionCheckResult represents the result of a permission check
type PermissionCheckResult struct {
	Path        string
	Exists      bool
	Readable    bool
	Writable    bool
	OwnerUID    int
	OwnerGID    int
	CurrentUID  int
	CurrentGID  int
	ErrorMsg    string
	FixCommands []string
}

// CheckDirectoryPermissions checks if a directory has correct permissions for the current user
func CheckDirectoryPermissions(path string) (*PermissionCheckResult, error) {
	result := &PermissionCheckResult{
		Path: path,
	}

	currentUser, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("failed to get current user: %w", err)
	}

	currentUID, _ := strconv.Atoi(currentUser.Uid)
	currentGID, _ := strconv.Atoi(currentUser.Gid)
	result.CurrentUID = currentUID
	result.CurrentGID = currentGID

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			result.Exists = false
			result.ErrorMsg = fmt.Sprintf("directory does not exist: %s", path)

			// Suggest fix commands based on current user
			if currentUID == 0 {
				result.FixCommands = []string{
					fmt.Sprintf("mkdir -p %s", path),
					fmt.Sprintf("chown -R 999:999 %s", path),
				}
			} else {
				result.FixCommands = []string{
					fmt.Sprintf("# Run as root to create directory:"),
					fmt.Sprintf("sudo mkdir -p %s", path),
					fmt.Sprintf("sudo chown -R 999:999 %s", path),
				}
			}
			return result, nil
		}
		return nil, fmt.Errorf("failed to stat directory: %w", err)
	}

	result.Exists = true

	stat := info.Sys().(*syscall.Stat_t)
	result.OwnerUID = int(stat.Uid)
	result.OwnerGID = int(stat.Gid)

	// Check if directory is readable
	result.Readable = info.Mode().Perm()&0400 != 0 || result.OwnerUID == currentUID

	// Check if directory is writable
	result.Writable = info.Mode().Perm()&0200 != 0 || result.OwnerUID == currentUID

	// Check for permission mismatches
	if result.OwnerUID != 999 || result.OwnerGID != 999 {
		result.ErrorMsg = fmt.Sprintf(
			"directory is owned by UID %d:GID %d, expected postgres user (999:999)",
			result.OwnerUID,
			result.OwnerGID,
		)

		// Generate fix commands
		if currentUID == 0 {
			result.FixCommands = []string{
				fmt.Sprintf("chown -R postgres:postgres %s", path),
			}
		} else if currentUID == result.OwnerUID {
			// User owns the directory but needs to run as root to fix
			result.FixCommands = []string{
				fmt.Sprintf("# Directory is owned by current user (UID %d), not postgres (999)", currentUID),
				fmt.Sprintf("# Option 1: Run container as root to fix permissions, then restart as postgres:"),
				fmt.Sprintf("docker run --user root -v <volume>:/var/lib/postgresql/data <image>"),
				fmt.Sprintf(""),
				fmt.Sprintf("# Option 2: Fix permissions on host (if using bind mount):"),
				fmt.Sprintf("sudo chown -R 999:999 %s", path),
				fmt.Sprintf(""),
				fmt.Sprintf("# Option 3: Use named volume (Docker handles permissions automatically):"),
				fmt.Sprintf("docker run -v pgdata:/var/lib/postgresql/data <image>"),
			}
		} else {
			result.FixCommands = []string{
				fmt.Sprintf("# Directory is owned by UID %d, not postgres (999) or current user (%d)", result.OwnerUID, currentUID),
				fmt.Sprintf("# Option 1: Run container explicitly as root to fix permissions:"),
				fmt.Sprintf("docker run --user root -v <volume>:/var/lib/postgresql/data <image>"),
				fmt.Sprintf(""),
				fmt.Sprintf("# Option 2: Fix permissions from host:"),
				fmt.Sprintf("docker run --rm --user root -v <volume>:/data alpine chown -R 999:999 /data"),
			}
		}
	} else if currentUID != 999 {
		result.ErrorMsg = fmt.Sprintf(
			"running as UID %d, but directory is owned by postgres (999). Container should run as postgres user",
			currentUID,
		)
		result.FixCommands = []string{
			"# Container is running as wrong user",
			"# Remove --user flag from docker run command to use default (postgres) user",
		}
	}

	return result, nil
}

// CheckPGDATAPermissions performs comprehensive permission checks on PGDATA
func CheckPGDATAPermissions(pgdataPath string) error {
	result, err := CheckDirectoryPermissions(pgdataPath)
	if err != nil {
		return fmt.Errorf("permission check failed: %w", err)
	}

	if !result.Exists {
		// Directory doesn't exist - this might be OK for first initialization
		return nil
	}

	if result.ErrorMsg != "" {
		return &PermissionError{
			Path:        result.Path,
			Message:     result.ErrorMsg,
			CurrentUID:  result.CurrentUID,
			OwnerUID:    result.OwnerUID,
			FixCommands: result.FixCommands,
		}
	}

	// Check if we can actually read and write
	if !result.Readable {
		return &PermissionError{
			Path:        result.Path,
			Message:     fmt.Sprintf("directory is not readable by current user (UID %d)", result.CurrentUID),
			CurrentUID:  result.CurrentUID,
			OwnerUID:    result.OwnerUID,
			FixCommands: []string{"# Permissions are too restrictive"},
		}
	}

	if !result.Writable {
		return &PermissionError{
			Path:        result.Path,
			Message:     fmt.Sprintf("directory is not writable by current user (UID %d)", result.CurrentUID),
			CurrentUID:  result.CurrentUID,
			OwnerUID:    result.OwnerUID,
			FixCommands: []string{"# Permissions are too restrictive"},
		}
	}

	return nil
}

// PermissionError represents a permission-related error with fix suggestions
type PermissionError struct {
	Path        string
	Message     string
	CurrentUID  int
	OwnerUID    int
	FixCommands []string
}

func (e *PermissionError) Error() string {
	msg := fmt.Sprintf("permission error for %s: %s\n", e.Path, e.Message)
	msg += fmt.Sprintf("\nCurrent user: UID %d\n", e.CurrentUID)
	msg += fmt.Sprintf("Directory owner: UID %d\n", e.OwnerUID)

	if len(e.FixCommands) > 0 {
		msg += "\nHow to fix:\n"
		for _, cmd := range e.FixCommands {
			msg += "  " + cmd + "\n"
		}
	}

	return msg
}

// GetCurrentUserInfo returns information about the current user
func GetCurrentUserInfo() (uid int, gid int, username string, err error) {
	currentUser, err := user.Current()
	if err != nil {
		return 0, 0, "", fmt.Errorf("failed to get current user: %w", err)
	}

	uid, _ = strconv.Atoi(currentUser.Uid)
	gid, _ = strconv.Atoi(currentUser.Gid)
	username = currentUser.Username

	return uid, gid, username, nil
}

// IsRunningAsRoot checks if the current process is running as root
func IsRunningAsRoot() bool {
	return os.Getuid() == 0
}

// IsRunningAsPostgres checks if the current process is running as postgres user (UID 999)
func IsRunningAsPostgres() bool {
	return os.Getuid() == 999
}
