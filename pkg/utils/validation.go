package utils

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Line    int
	Column  int
	Message string
	Raw     string
}

// BinaryValidator defines the interface for validating configurations with binaries
type BinaryValidator interface {
	GetBinaryPath() string
	GetValidationArgs(configPath string) []string
	ParseValidationError(output string) (*ValidationError, error)
}

// ValidateWithBinary validates a configuration file using a binary tool
func ValidateWithBinary(validator BinaryValidator, configPath string) error {
	binaryPath := validator.GetBinaryPath()
	args := validator.GetValidationArgs(configPath)

	cmd := exec.Command(binaryPath, args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		validationErr, parseErr := validator.ParseValidationError(string(output))
		if parseErr != nil {
			return fmt.Errorf("validation failed: %w\nOutput: %s", err, output)
		}
		if validationErr != nil {
			return fmt.Errorf("validation error at line %d, column %d: %s",
				validationErr.Line, validationErr.Column, validationErr.Message)
		}
		return fmt.Errorf("validation failed: %w", err)
	}

	return nil
}

// ParseErrorWithRegex is a helper function to parse validation errors using regex patterns
func ParseErrorWithRegex(output string, patterns map[string]*regexp.Regexp) (*ValidationError, error) {
	lines := strings.Split(output, "\n")
	validationErr := &ValidationError{
		Raw: output,
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Try to match line pattern
		if linePattern, exists := patterns["line"]; exists {
			if matches := linePattern.FindStringSubmatch(line); len(matches) > 1 {
				fmt.Sscanf(matches[1], "%d", &validationErr.Line)
			}
		}

		// Try to match column pattern
		if colPattern, exists := patterns["column"]; exists {
			if matches := colPattern.FindStringSubmatch(line); len(matches) > 1 {
				fmt.Sscanf(matches[1], "%d", &validationErr.Column)
			}
		}

		// Try to match error message pattern
		if msgPattern, exists := patterns["message"]; exists {
			if matches := msgPattern.FindStringSubmatch(line); len(matches) > 1 {
				validationErr.Message = matches[1]
			}
		}

		// Try to match combined pattern (line, column, message)
		if combinedPattern, exists := patterns["combined"]; exists {
			if matches := combinedPattern.FindStringSubmatch(line); len(matches) > 3 {
				fmt.Sscanf(matches[1], "%d", &validationErr.Line)
				fmt.Sscanf(matches[2], "%d", &validationErr.Column)
				validationErr.Message = matches[3]
			}
		}
	}

	if validationErr.Message == "" {
		validationErr.Message = strings.TrimSpace(output)
	}

	return validationErr, nil
}
