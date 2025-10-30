package utils

import (
	"encoding/json"
	"fmt"
	"os"
)

// SensitiveString is a string type that hides sensitive information in logs and JSON output
type SensitiveString string

// String implements fmt.Stringer to prevent accidental logging of sensitive values
func (s SensitiveString) String() string {
	return "[REDACTED]"
}

// GoString implements fmt.GoStringer to prevent accidental logging in %#v format
func (s SensitiveString) GoString() string {
	return "[REDACTED]"
}

// MarshalJSON redacts the value in JSON output
func (s SensitiveString) MarshalJSON() ([]byte, error) {
	return json.Marshal("[REDACTED]")
}

// MarshalText redacts the value in text marshaling (used by many serializers)
func (s SensitiveString) MarshalText() ([]byte, error) {
	return []byte("[REDACTED]"), nil
}

// Value returns the actual string value - use with caution and only when necessary
func (s SensitiveString) Value() string {
	return string(s)
}

// IsEmpty returns true if the sensitive string is empty
func (s SensitiveString) IsEmpty() bool {
	return string(s) == ""
}

// Equals compares two sensitive strings for equality (constant-time comparison)
func (s SensitiveString) Equals(other SensitiveString) bool {
	// Use subtle.ConstantTimeCompare for security
	a := []byte(s)
	b := []byte(other)

	if len(a) != len(b) {
		return false
	}

	result := 0
	for i := 0; i < len(a); i++ {
		result |= int(a[i] ^ b[i])
	}

	return result == 0
}

// NewSensitiveString creates a new SensitiveString from a regular string
func NewSensitiveString(value string) SensitiveString {

	return SensitiveString(value)
}

// SensitiveStringFromEnv creates a SensitiveString from an environment variable
// Returns empty SensitiveString if the environment variable doesn't exist
func SensitiveStringFromEnv(key string) SensitiveString {
	if value := getEnvString(key, ""); value != "" {
		return SensitiveString(value)
	}
	return SensitiveString("")
}

// Format implements fmt.Formatter to ensure sensitive values are redacted in all format verbs
func (s SensitiveString) Format(f fmt.State, verb rune) {
	switch verb {
	case 'v':
		if f.Flag('#') {
			fmt.Fprint(f, "[REDACTED]")
		} else {
			fmt.Fprint(f, "[REDACTED]")
		}
	case 's':
		fmt.Fprint(f, "[REDACTED]")
	case 'q':
		fmt.Fprint(f, `"[REDACTED]"`)
	default:
		fmt.Fprint(f, "[REDACTED]")
	}
}

// getEnvString gets a string environment variable with a default value
func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
