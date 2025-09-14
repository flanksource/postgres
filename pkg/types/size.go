package types

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/flanksource/postgres/pkg/utils"
)

// Size represents a memory or storage size that can be parsed from various string formats
// and provides type-safe operations. It stores the size internally as bytes.
type Size uint64

// ParseSize creates a Size from a string representation (e.g., "128MB", "1GB", "512kB")
func ParseSize(s string) (Size, error) {
	if s == "" {
		return Size(0), nil
	}
	
	bytes, err := utils.ParseSize(s)
	if err != nil {
		return Size(0), fmt.Errorf("invalid size format: %w", err)
	}
	
	return Size(bytes), nil
}

// Bytes returns the size in bytes
func (s Size) Bytes() uint64 {
	return uint64(s)
}

// KB returns the size in kilobytes
func (s Size) KB() uint64 {
	return uint64(s) / utils.KB
}

// MB returns the size in megabytes  
func (s Size) MB() uint64 {
	return uint64(s) / utils.MB
}

// GB returns the size in gigabytes
func (s Size) GB() uint64 {
	return uint64(s) / utils.GB
}

// String returns a human-readable string representation
func (s Size) String() string {
	return utils.FormatSize(uint64(s))
}

// PostgreSQLString returns a PostgreSQL-compatible string representation
func (s Size) PostgreSQLString() string {
	return utils.FormatSizePostgreSQL(uint64(s))
}

// PostgreSQLMB returns a PostgreSQL-compatible string representation in MB units
func (s Size) PostgreSQLMB() string {
	return utils.FormatSizePostgreSQLMB(uint64(s))
}

// MarshalJSON implements json.Marshaler interface
func (s Size) MarshalJSON() ([]byte, error) {
	// Use PostgreSQL-compatible format for JSON to avoid decimals
	return json.Marshal(s.PostgreSQLString())
}

// UnmarshalJSON implements json.Unmarshaler interface
func (s *Size) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		// Try to unmarshal as number (assume bytes)
		var num uint64
		if numErr := json.Unmarshal(data, &num); numErr != nil {
			return fmt.Errorf("size must be a string or number: %w", err)
		}
		*s = Size(num)
		return nil
	}
	
	parsed, err := ParseSize(str)
	if err != nil {
		return err
	}
	
	*s = parsed
	return nil
}

// MarshalYAML implements yaml.Marshaler interface
func (s Size) MarshalYAML() (interface{}, error) {
	return s.String(), nil
}

// UnmarshalYAML implements yaml.Unmarshaler interface  
func (s *Size) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var str string
	if err := unmarshal(&str); err != nil {
		// Try to unmarshal as number (assume bytes)
		var num uint64
		if numErr := unmarshal(&num); numErr != nil {
			return fmt.Errorf("size must be a string or number: %w", err)
		}
		*s = Size(num)
		return nil
	}
	
	parsed, err := ParseSize(str)
	if err != nil {
		return err
	}
	
	*s = parsed
	return nil
}

// MarshalText implements encoding.TextMarshaler interface (for koanf)
func (s Size) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler interface (for koanf)
func (s *Size) UnmarshalText(text []byte) error {
	str := strings.TrimSpace(string(text))
	if str == "" {
		*s = Size(0)
		return nil
	}
	
	// Handle plain numbers (assume bytes)
	if num, err := strconv.ParseUint(str, 10, 64); err == nil {
		*s = Size(num)
		return nil
	}
	
	parsed, err := ParseSize(str)
	if err != nil {
		return err
	}
	
	*s = parsed
	return nil
}

// IsZero returns true if the size is zero
func (s Size) IsZero() bool {
	return s == 0
}

// Add adds another size to this one
func (s Size) Add(other Size) Size {
	return Size(uint64(s) + uint64(other))
}

// Sub subtracts another size from this one
func (s Size) Sub(other Size) Size {
	if other > s {
		return Size(0)
	}
	return Size(uint64(s) - uint64(other))
}

// Mul multiplies the size by a factor
func (s Size) Mul(factor float64) Size {
	return Size(uint64(float64(s) * factor))
}

// Div divides the size by a factor
func (s Size) Div(factor float64) Size {
	if factor == 0 {
		return Size(0)
	}
	return Size(uint64(float64(s) / factor))
}

// MustParseSize parses a size string and panics on error
func MustParseSize(s string) Size {
	size, err := ParseSize(s)
	if err != nil {
		panic(fmt.Sprintf("invalid size: %v", err))
	}
	return size
}

// NewSize creates a Size from a string, intended for known-valid values
func NewSize(s string) Size {
	size, err := ParseSize(s)
	if err != nil {
		// For known-valid strings used in defaults, this should not happen
		panic(fmt.Sprintf("NewSize: invalid size string %q: %v", s, err))
	}
	return size
}