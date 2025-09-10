package types

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/flanksource/postgres/pkg/utils"
)

// Duration represents a time duration that can be parsed from various string formats
// and provides type-safe operations. It stores the duration internally as time.Duration.
type Duration time.Duration

// ParseDuration creates a Duration from a string representation (e.g., "5min", "30s", "1h")
func ParseDuration(s string) (Duration, error) {
	if s == "" || s == "0" {
		return Duration(0), nil
	}
	
	d, err := utils.ParseDuration(s)
	if err != nil {
		return Duration(0), fmt.Errorf("invalid duration format: %w", err)
	}
	
	return Duration(d), nil
}

// Duration returns the underlying time.Duration
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// Nanoseconds returns the duration as nanoseconds
func (d Duration) Nanoseconds() int64 {
	return int64(d)
}

// Microseconds returns the duration as microseconds
func (d Duration) Microseconds() int64 {
	return int64(d) / int64(time.Microsecond)
}

// Milliseconds returns the duration as milliseconds
func (d Duration) Milliseconds() int64 {
	return int64(d) / int64(time.Millisecond)
}

// Seconds returns the duration as seconds
func (d Duration) Seconds() float64 {
	return time.Duration(d).Seconds()
}

// Minutes returns the duration as minutes
func (d Duration) Minutes() float64 {
	return time.Duration(d).Minutes()
}

// Hours returns the duration as hours
func (d Duration) Hours() float64 {
	return time.Duration(d).Hours()
}

// String returns a human-readable string representation
func (d Duration) String() string {
	if d == 0 {
		return "0"
	}
	return utils.FormatDuration(time.Duration(d))
}

// PostgreSQLString returns a PostgreSQL-compatible string representation
func (d Duration) PostgreSQLString() string {
	if d == 0 {
		return "0"
	}
	return utils.FormatDurationPostgreSQL(time.Duration(d))
}

// MarshalJSON implements json.Marshaler interface
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// UnmarshalJSON implements json.Unmarshaler interface
func (d *Duration) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		// Try to unmarshal as number (assume milliseconds for PostgreSQL compatibility)
		var num int64
		if numErr := json.Unmarshal(data, &num); numErr != nil {
			return fmt.Errorf("duration must be a string or number: %w", err)
		}
		*d = Duration(time.Duration(num) * time.Millisecond)
		return nil
	}
	
	parsed, err := ParseDuration(str)
	if err != nil {
		return err
	}
	
	*d = parsed
	return nil
}

// MarshalYAML implements yaml.Marshaler interface
func (d Duration) MarshalYAML() (interface{}, error) {
	return d.String(), nil
}

// UnmarshalYAML implements yaml.Unmarshaler interface
func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var str string
	if err := unmarshal(&str); err != nil {
		// Try to unmarshal as number (assume milliseconds for PostgreSQL compatibility)
		var num int64
		if numErr := unmarshal(&num); numErr != nil {
			return fmt.Errorf("duration must be a string or number: %w", err)
		}
		*d = Duration(time.Duration(num) * time.Millisecond)
		return nil
	}
	
	parsed, err := ParseDuration(str)
	if err != nil {
		return err
	}
	
	*d = parsed
	return nil
}

// MarshalText implements encoding.TextMarshaler interface (for koanf)
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler interface (for koanf)
func (d *Duration) UnmarshalText(text []byte) error {
	str := strings.TrimSpace(string(text))
	if str == "" || str == "0" {
		*d = Duration(0)
		return nil
	}
	
	// Handle plain numbers (assume milliseconds for PostgreSQL compatibility)
	if num, err := strconv.ParseInt(str, 10, 64); err == nil {
		*d = Duration(time.Duration(num) * time.Millisecond)
		return nil
	}
	
	parsed, err := ParseDuration(str)
	if err != nil {
		return err
	}
	
	*d = parsed
	return nil
}

// IsZero returns true if the duration is zero
func (d Duration) IsZero() bool {
	return d == 0
}

// Add adds another duration to this one
func (d Duration) Add(other Duration) Duration {
	return Duration(time.Duration(d) + time.Duration(other))
}

// Sub subtracts another duration from this one
func (d Duration) Sub(other Duration) Duration {
	result := time.Duration(d) - time.Duration(other)
	if result < 0 {
		return Duration(0)
	}
	return Duration(result)
}

// Mul multiplies the duration by a factor
func (d Duration) Mul(factor float64) Duration {
	return Duration(time.Duration(float64(d) * factor))
}

// Div divides the duration by a factor
func (d Duration) Div(factor float64) Duration {
	if factor == 0 {
		return Duration(0)
	}
	return Duration(time.Duration(float64(d) / factor))
}

// Truncate truncates the duration to the specified precision
func (d Duration) Truncate(precision time.Duration) Duration {
	return Duration(time.Duration(d).Truncate(precision))
}

// Round rounds the duration to the nearest multiple of precision
func (d Duration) Round(precision time.Duration) Duration {
	return Duration(time.Duration(d).Round(precision))
}

// MustParseDuration parses a duration string and panics on error
func MustParseDuration(s string) Duration {
	duration, err := ParseDuration(s)
	if err != nil {
		panic(fmt.Sprintf("invalid duration: %v", err))
	}
	return duration
}

// NewDuration creates a Duration from a string, intended for known-valid values
func NewDuration(s string) Duration {
	duration, err := ParseDuration(s)
	if err != nil {
		// For known-valid strings used in defaults, this should not happen
		panic(fmt.Sprintf("NewDuration: invalid duration string %q: %v", s, err))
	}
	return duration
}