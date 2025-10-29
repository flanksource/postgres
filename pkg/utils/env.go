package utils

import (
	"fmt"
	"os"
	"strings"
)

// FileEnv retrieves an environment variable value or reads from a file if *_FILE variant exists.
// This implements Docker secrets pattern where sensitive values can be read from files.
// Returns error if both env var and file variant are set (exclusive use required).
func FileEnv(key, defaultVal string) (string, error) {
	envVal := os.Getenv(key)
	fileKey := key + "_FILE"
	fileVal := os.Getenv(fileKey)

	if envVal != "" && fileVal != "" {
		return "", fmt.Errorf("both %s and %s are set, only one should be used", key, fileKey)
	}

	if envVal != "" {
		return envVal, nil
	}

	if fileVal != "" {
		content, err := os.ReadFile(fileVal)
		if err != nil {
			return "", fmt.Errorf("failed to read %s from file %s: %w", key, fileVal, err)
		}
		return strings.TrimSpace(string(content)), nil
	}

	return defaultVal, nil
}

// MustFileEnv is like FileEnv but panics on error
func MustFileEnv(key, defaultVal string) string {
	val, err := FileEnv(key, defaultVal)
	if err != nil {
		panic(err)
	}
	return val
}
