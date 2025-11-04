package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func LoadPostmasterOpts(path string) (Conf, error) {
	opts := Conf{}
	data, err := os.ReadFile(path)
	if err != nil {
		return opts, err
	}

	for _, arg := range strings.Fields(string(data)) {
		if strings.HasPrefix(arg, "--") {
			parts := strings.SplitN(arg[2:], "=", 2)
			paramName := parts[0]
			paramValue := ""
			if len(parts) > 1 {
				paramValue = parts[1]
			}
			opts[paramName] = paramValue
		}
	}

	return opts, nil

}

func LoadConfFile(path string) (Conf, error) {

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Conf{}, nil
	}
	if err != nil {
		panic(err)
		// return nil, err
	}
	lines := strings.Split(string(data), "\n")
	conf := Conf{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if commentIdx := strings.Index(value, "#"); commentIdx != -1 {
			value = value[:commentIdx]
		}

		value = strings.TrimSpace(value)

		value = strings.Trim(value, "'\"")

		conf[key] = value
	}
	return conf, nil
}

// EnsureIncludeDirective ensures that postgresql.conf includes the specified file
// If the include directive doesn't exist, it will be added at the end
func EnsureIncludeDirective(postgresConfPath, includeFile string) error {

	if _, err := os.Stat(postgresConfPath); os.IsNotExist(err) {
		if _, err := os.Stat(filepath.Dir(postgresConfPath)); err == nil {
			// return error with a list of files in the data directory
			files, _ := os.ReadDir(filepath.Dir(postgresConfPath))
			var fileNames []string
			for _, f := range files {
				fileNames = append(fileNames, f.Name())
			}
			return fmt.Errorf("postgresql.conf does not exist at path: %s, data directory contains: %v", postgresConfPath, fileNames)
		}
		return fmt.Errorf("postgresql.conf file does not exist at path: %s", postgresConfPath)
	}
	data, err := os.ReadFile(postgresConfPath)
	if err != nil {
		return fmt.Errorf("failed to read postgresql.conf: %w", err)
	}

	lines := strings.Split(string(data), "\n")

	// Check if include directive already exists
	includePattern := fmt.Sprintf("include '%s'", includeFile)
	includeIfExistsPattern := fmt.Sprintf("include_if_exists '%s'", includeFile)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip comments
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Check for existing include directive
		if strings.Contains(trimmed, includePattern) || strings.Contains(trimmed, includeIfExistsPattern) {
			// Include already exists
			return nil
		}
	}

	// Include not found, append it
	file, err := os.OpenFile(postgresConfPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open postgresql.conf for writing: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	// Add a newline if file doesn't end with one
	if len(data) > 0 && data[len(data)-1] != '\n' {
		writer.WriteString("\n")
	}

	// Add the include directive
	writer.WriteString("\n")
	writer.WriteString("# Include pg_tune optimizations\n")
	writer.WriteString(fmt.Sprintf("include_if_exists '%s'\n", includeFile))

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to write to postgresql.conf: %w", err)
	}

	return nil
}
