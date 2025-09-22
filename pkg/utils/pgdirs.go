package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// PostgreSQLDirs holds detected PostgreSQL directories
type PostgreSQLDirs struct {
	BinDir  string
	DataDir string
}

// DetectPostgreSQLDirs detects both binary and data directories
func DetectPostgreSQLDirs() (*PostgreSQLDirs, error) {
	binDir, err := DetectBinDir()
	if err != nil {
		return nil, fmt.Errorf("failed to detect binary directory: %w", err)
	}

	dataDir, err := DetectDataDir()
	if err != nil {
		return nil, fmt.Errorf("failed to detect data directory: %w", err)
	}

	return &PostgreSQLDirs{
		BinDir:  binDir,
		DataDir: dataDir,
	}, nil
}

// DetectBinDir detects PostgreSQL binary directory
func DetectBinDir() (string, error) {
	// 1. Check PGBIN environment variable
	if pgbin := os.Getenv("PGBIN"); pgbin != "" {
		if isValidBinDir(pgbin) {
			return pgbin, nil
		}
	}

	// 2. Check if postgres is in PATH
	if pgPath, err := exec.LookPath("postgres"); err == nil {
		binDir := filepath.Dir(pgPath)
		if isValidBinDir(binDir) {
			return binDir, nil
		}
	}

	// 3. Detect from running postgres process
	if binDir, _ := detectFromProcess(); binDir != "" {
		if isValidBinDir(binDir) {
			return binDir, nil
		}
	}

	// 4. Check common locations by OS
	commonPaths := getCommonBinPaths()
	for _, path := range commonPaths {
		if isValidBinDir(path) {
			return path, nil
		}

		// Handle wildcard paths
		if strings.Contains(path, "*") {
			matches, err := filepath.Glob(path)
			if err == nil {
				for _, match := range matches {
					if isValidBinDir(match) {
						return match, nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("PostgreSQL binary directory not found. Checked PATH and common locations")
}

// DetectDataDir detects PostgreSQL data directory
func DetectDataDir() (string, error) {
	// 1. Check PGDATA environment variable
	if pgdata := os.Getenv("PGDATA"); pgdata != "" {
		if isValidDataDir(pgdata) {
			return pgdata, nil
		}
	}

	// 2. Detect from running postgres process
	if _, dataDir := detectFromProcess(); dataDir != "" {
		if isValidDataDir(dataDir) {
			return dataDir, nil
		}
	}

	// 3. Check common locations by OS
	commonPaths := getCommonDataPaths()
	for _, path := range commonPaths {
		if isValidDataDir(path) {
			return path, nil
		}

		// Handle wildcard paths
		if strings.Contains(path, "*") {
			matches, err := filepath.Glob(path)
			if err == nil {
				for _, match := range matches {
					if isValidDataDir(match) {
						return match, nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("PostgreSQL data directory not found. Checked PGDATA and common locations")
}

// detectFromProcess attempts to detect directories from running postgres process
func detectFromProcess() (binDir, dataDir string) {
	// Try to find postgres process and extract paths
	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return "", ""
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "postgres") && strings.Contains(line, "-D") {
			// Parse postgres command line to extract -D flag (data directory)
			fields := strings.Fields(line)
			for i, field := range fields {
				if field == "-D" && i+1 < len(fields) {
					dataDir = fields[i+1]
					break
				}
				if strings.HasPrefix(field, "-D") {
					dataDir = strings.TrimPrefix(field, "-D")
					break
				}
			}

			// Extract binary directory from the postgres executable path
			for _, field := range fields {
				if strings.HasSuffix(field, "postgres") || strings.Contains(field, "/postgres") {
					if filepath.Base(field) == "postgres" {
						binDir = filepath.Dir(field)
						break
					}
				}
			}

			if dataDir != "" {
				break
			}
		}
	}

	return binDir, dataDir
}

// getCommonBinPaths returns common PostgreSQL binary paths by OS
func getCommonBinPaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/usr/local/pgsql/bin",
			"/opt/homebrew/bin",
			"/opt/homebrew/opt/postgresql*/bin",
			"/usr/local/opt/postgresql*/bin",
			"/Applications/Postgres.app/Contents/Versions/*/bin",
			"/Library/PostgreSQL/*/bin",
		}
	case "linux":
		return []string{
			"/usr/lib/postgresql/*/bin",
			"/usr/pgsql-*/bin",
			"/opt/postgresql/*/bin",
			"/usr/local/pgsql/bin",
			"/opt/pgsql/bin",
			"/usr/bin", // Some distros put postgres in /usr/bin
		}
	case "windows":
		return []string{
			"C:\\Program Files\\PostgreSQL\\*\\bin",
			"C:\\PostgreSQL\\*\\bin",
		}
	default:
		return []string{
			"/usr/local/pgsql/bin",
			"/opt/postgresql/bin",
		}
	}
}

// getCommonDataPaths returns common PostgreSQL data paths by OS
func getCommonDataPaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/usr/local/var/postgres",
			"/opt/homebrew/var/postgres",
			"/opt/homebrew/var/postgresql*",
			"/usr/local/var/postgresql*",
			"/Library/PostgreSQL/*/data",
		}
	case "linux":
		return []string{
			"/var/lib/postgresql/*/main",
			"/var/lib/pgsql/data",
			"/usr/local/pgsql/data",
			"/opt/postgresql/*/data",
			"/var/lib/postgresql/data",
		}
	case "windows":
		return []string{
			"C:\\Program Files\\PostgreSQL\\*\\data",
			"C:\\PostgreSQL\\*\\data",
		}
	default:
		return []string{
			"/var/lib/postgresql/data",
			"/usr/local/pgsql/data",
		}
	}
}

// isValidBinDir checks if a directory contains PostgreSQL binaries
func isValidBinDir(dir string) bool {
	if dir == "" {
		return false
	}

	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return false
	}

	// Check for postgres binary
	postgresPath := filepath.Join(dir, "postgres")
	if runtime.GOOS == "windows" {
		postgresPath += ".exe"
	}

	if _, err := os.Stat(postgresPath); err == nil {
		return true
	}

	return false
}

// isValidDataDir checks if a directory is a valid PostgreSQL data directory
func isValidDataDir(dir string) bool {
	if dir == "" {
		return false
	}

	// Check if directory exists
	if stat, err := os.Stat(dir); os.IsNotExist(err) || !stat.IsDir() {
		return false
	}

	// Check for PG_VERSION file (indicates a PostgreSQL data directory)
	pgVersionPath := filepath.Join(dir, "PG_VERSION")
	if _, err := os.Stat(pgVersionPath); err == nil {
		return true
	}

	return false
}

// GetPostgreSQLVersion reads the PostgreSQL version from PG_VERSION file
func GetPostgreSQLVersion(dataDir string) (string, error) {
	pgVersionPath := filepath.Join(dataDir, "PG_VERSION")
	content, err := os.ReadFile(pgVersionPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}