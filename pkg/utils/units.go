package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Size constants in bytes
const (
	Byte = 1
	KB   = 1024 * Byte
	MB   = 1024 * KB
	GB   = 1024 * MB
	TB   = 1024 * GB
)

// Time constants
const (
	Microsecond = time.Microsecond
	Millisecond = time.Millisecond
	Second      = time.Second
	Minute      = 60 * Second
	Hour        = 60 * Minute
	Day         = 24 * Hour
)

// sizeRegex matches size strings like "128MB", "1GB", "512kB"
var sizeRegex = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*([KMGT]?B)$`)

// durationRegex matches duration strings like "5min", "30s", "1h"
var durationRegex = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*(us|ms|s|min|h|d)$`)

// ParseSize parses a size string and returns the size in bytes
func ParseSize(sizeStr string) (uint64, error) {
	if sizeStr == "" {
		return 0, fmt.Errorf("empty size string")
	}

	// Handle plain numbers (assume bytes)
	if val, err := strconv.ParseUint(sizeStr, 10, 64); err == nil {
		return val, nil
	}

	matches := sizeRegex.FindStringSubmatch(strings.ToUpper(strings.TrimSpace(sizeStr)))
	if matches == nil {
		return 0, fmt.Errorf("invalid size format: %s", sizeStr)
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size value: %s", matches[1])
	}

	unit := matches[2]
	var multiplier uint64

	switch unit {
	case "B":
		multiplier = Byte
	case "KB":
		multiplier = KB
	case "MB":
		multiplier = MB
	case "GB":
		multiplier = GB
	case "TB":
		multiplier = TB
	default:
		return 0, fmt.Errorf("unknown size unit: %s", unit)
	}

	return uint64(value * float64(multiplier)), nil
}

// FormatSize formats a size in bytes to a human-readable string
func FormatSize(bytes uint64) string {
	if bytes == 0 {
		return "0B"
	}

	if bytes >= TB {
		return fmt.Sprintf("%.1fTB", float64(bytes)/float64(TB))
	}
	if bytes >= GB {
		return fmt.Sprintf("%.1fGB", float64(bytes)/float64(GB))
	}
	if bytes >= MB {
		return fmt.Sprintf("%.1fMB", float64(bytes)/float64(MB))
	}
	if bytes >= KB {
		return fmt.Sprintf("%.1fkB", float64(bytes)/float64(KB))
	}

	return fmt.Sprintf("%dB", bytes)
}

// FormatSizePostgreSQL formats a size in bytes as PostgreSQL expects it
func FormatSizePostgreSQL(bytes uint64) string {
	if bytes == 0 {
		return "0"
	}

	// PostgreSQL prefers whole numbers, so we'll use the largest unit that gives a whole number
	if bytes >= TB && bytes%TB == 0 {
		return fmt.Sprintf("%dTB", bytes/TB)
	}
	if bytes >= GB && bytes%GB == 0 {
		return fmt.Sprintf("%dGB", bytes/GB)
	}
	if bytes >= MB && bytes%MB == 0 {
		return fmt.Sprintf("%dMB", bytes/MB)
	}
	if bytes >= KB && bytes%KB == 0 {
		return fmt.Sprintf("%dkB", bytes/KB)
	}

	// If we can't get a whole number, use the most appropriate unit
	if bytes >= TB {
		return fmt.Sprintf("%.0fTB", float64(bytes)/float64(TB))
	}
	if bytes >= GB {
		return fmt.Sprintf("%.0fGB", float64(bytes)/float64(GB))
	}
	if bytes >= MB {
		return fmt.Sprintf("%.0fMB", float64(bytes)/float64(MB))
	}
	if bytes >= KB {
		return fmt.Sprintf("%.0fkB", float64(bytes)/float64(KB))
	}

	return fmt.Sprintf("%d", bytes)
}

// FormatSizePostgreSQLMB formats a size in bytes as PostgreSQL MB units only
// This ensures consistent MB units for all PostgreSQL memory settings
func FormatSizePostgreSQLMB(bytes uint64) string {
	if bytes == 0 {
		return "0MB"
	}
	// Always convert to MB and round to nearest MB
	mb := (bytes + MB/2) / MB // Round to nearest MB
	if mb == 0 {
		mb = 1 // Minimum 1MB
	}
	return fmt.Sprintf("%dMB", mb)
}

// ParseDuration parses a duration string and returns the duration
func ParseDuration(durationStr string) (time.Duration, error) {
	if durationStr == "" {
		return 0, fmt.Errorf("empty duration string")
	}

	// Handle "0" as no timeout
	if durationStr == "0" {
		return 0, nil
	}

	// Handle plain numbers (assume milliseconds for PostgreSQL compatibility)
	if val, err := strconv.ParseInt(durationStr, 10, 64); err == nil {
		return time.Duration(val) * Millisecond, nil
	}

	matches := durationRegex.FindStringSubmatch(strings.ToLower(strings.TrimSpace(durationStr)))
	if matches == nil {
		return 0, fmt.Errorf("invalid duration format: %s", durationStr)
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid duration value: %s", matches[1])
	}

	unit := matches[2]
	var duration time.Duration

	switch unit {
	case "us":
		duration = time.Duration(value * float64(Microsecond))
	case "ms":
		duration = time.Duration(value * float64(Millisecond))
	case "s":
		duration = time.Duration(value * float64(Second))
	case "min":
		duration = time.Duration(value * float64(Minute))
	case "h":
		duration = time.Duration(value * float64(Hour))
	case "d":
		duration = time.Duration(value * float64(Day))
	default:
		return 0, fmt.Errorf("unknown duration unit: %s", unit)
	}

	return duration, nil
}

// FormatDuration formats a duration to a human-readable string
func FormatDuration(d time.Duration) string {
	if d == 0 {
		return "0"
	}

	if d >= Day {
		return fmt.Sprintf("%.1fd", float64(d)/float64(Day))
	}
	if d >= Hour {
		return fmt.Sprintf("%.1fh", float64(d)/float64(Hour))
	}
	if d >= Minute {
		return fmt.Sprintf("%.1fmin", float64(d)/float64(Minute))
	}
	if d >= Second {
		return fmt.Sprintf("%.1fs", float64(d)/float64(Second))
	}
	if d >= Millisecond {
		return fmt.Sprintf("%.1fms", float64(d)/float64(Millisecond))
	}

	return fmt.Sprintf("%.1fus", float64(d)/float64(Microsecond))
}

// FormatDurationPostgreSQL formats a duration as PostgreSQL expects it
func FormatDurationPostgreSQL(d time.Duration) string {
	if d == 0 {
		return "0"
	}

	// PostgreSQL prefers whole numbers, so we'll use the largest unit that gives a whole number
	if d >= Day && d%Day == 0 {
		return fmt.Sprintf("%dd", int64(d/Day))
	}
	if d >= Hour && d%Hour == 0 {
		return fmt.Sprintf("%dh", int64(d/Hour))
	}
	if d >= Minute && d%Minute == 0 {
		return fmt.Sprintf("%dmin", int64(d/Minute))
	}
	if d >= Second && d%Second == 0 {
		return fmt.Sprintf("%ds", int64(d/Second))
	}
	if d >= Millisecond && d%Millisecond == 0 {
		return fmt.Sprintf("%dms", int64(d/Millisecond))
	}
	if d%Microsecond == 0 {
		return fmt.Sprintf("%dus", int64(d/Microsecond))
	}

	// If we can't get a whole number, use the most appropriate unit
	if d >= Day {
		return fmt.Sprintf("%.0fd", float64(d)/float64(Day))
	}
	if d >= Hour {
		return fmt.Sprintf("%.0fh", float64(d)/float64(Hour))
	}
	if d >= Minute {
		return fmt.Sprintf("%.0fmin", float64(d)/float64(Minute))
	}
	if d >= Second {
		return fmt.Sprintf("%.0fs", float64(d)/float64(Second))
	}
	if d >= Millisecond {
		return fmt.Sprintf("%.0fms", float64(d)/float64(Millisecond))
	}

	return fmt.Sprintf("%.0fus", float64(d)/float64(Microsecond))
}

// BytesToKB converts bytes to kilobytes
func BytesToKB(bytes uint64) uint64 {
	return bytes / KB
}

// KBToBytes converts kilobytes to bytes
func KBToBytes(kb uint64) uint64 {
	return kb * KB
}

// BytesToMB converts bytes to megabytes
func BytesToMB(bytes uint64) uint64 {
	return bytes / MB
}

// MBToBytes converts megabytes to bytes
func MBToBytes(mb uint64) uint64 {
	return mb * MB
}

// BytesToGB converts bytes to gigabytes
func BytesToGB(bytes uint64) uint64 {
	return bytes / GB
}

// GBToBytes converts gigabytes to bytes
func GBToBytes(gb uint64) uint64 {
	return gb * GB
}

// ConvertSizeUnit converts a size from one unit to another
func ConvertSizeUnit(value uint64, fromUnit, toUnit string) (uint64, error) {
	// Convert to bytes first
	var bytes uint64
	switch strings.ToUpper(fromUnit) {
	case "B":
		bytes = value
	case "KB":
		bytes = value * KB
	case "MB":
		bytes = value * MB
	case "GB":
		bytes = value * GB
	case "TB":
		bytes = value * TB
	default:
		return 0, fmt.Errorf("unknown source unit: %s", fromUnit)
	}

	// Convert from bytes to target unit
	switch strings.ToUpper(toUnit) {
	case "B":
		return bytes, nil
	case "KB":
		return bytes / KB, nil
	case "MB":
		return bytes / MB, nil
	case "GB":
		return bytes / GB, nil
	case "TB":
		return bytes / TB, nil
	default:
		return 0, fmt.Errorf("unknown target unit: %s", toUnit)
	}
}

// IsValidSizeString checks if a string is a valid size format
func IsValidSizeString(s string) bool {
	_, err := ParseSize(s)
	return err == nil
}

// IsValidDurationString checks if a string is a valid duration format
func IsValidDurationString(s string) bool {
	_, err := ParseDuration(s)
	return err == nil
}

// FormatPercent formats a float as a percentage string
func FormatPercent(value float64) string {
	return fmt.Sprintf("%.1f", value)
}

// FormatBoolean formats a boolean for PostgreSQL configuration
func FormatBoolean(value bool) string {
	if value {
		return "on"
	}
	return "off"
}
