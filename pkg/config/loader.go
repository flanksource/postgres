package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/flanksource/postgres/pkg"
)

// LoadPostgresConf loads PostgreSQL configuration from a JSON file
func LoadPostgresConf(filename string) (*pkg.PostgresConf, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config pkg.PostgresConf
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	return &config, nil
}

// DefaultPostgresConf returns a PostgresConf with sensible defaults
func DefaultPostgresConf() *pkg.PostgresConf {
	return &pkg.PostgresConf{
		Port:            5432,
		ListenAddresses: "localhost",
	}
}

// LoadSettingsFromQuery parses pg_settings query results into ConfSettings
func LoadSettingsFromQuery(records []map[string]any) (ConfSettings, error) {
	settings := ConfSettings{}

	for _, record := range records {
		name := getStringField(record, "name")
		if name == "" {
			continue
		}

		setting := ConfigSetting{
			Name:           name,
			Setting:        getStringField(record, "setting"),
			Unit:           getNullableString(record, "unit"),
			Category:       getStringField(record, "category"),
			ShortDesc:      getStringField(record, "short_desc"),
			ExtraDesc:      getNullableString(record, "extra_desc"),
			Context:        getStringField(record, "context"),
			Vartype:        getStringField(record, "vartype"),
			Source:         getStringField(record, "source"),
			MinVal:         getNullableString(record, "min_val"),
			MaxVal:         getNullableString(record, "max_val"),
			Enumvals:       getStringArray(record, "enumvals"),
			BootVal:        getStringField(record, "boot_val"),
			ResetVal:       getStringField(record, "reset_val"),
			Sourcefile:     getNullableString(record, "sourcefile"),
			Sourceline:     getNullableInt(record, "sourceline"),
			PendingRestart: getBoolField(record, "pending_restart"),
		}

		settings = append(settings, setting)
	}

	return settings, nil
}

func getStringField(record map[string]any, field string) string {
	if val, ok := record[field]; ok && val != nil {
		return strings.TrimSpace(fmt.Sprintf("%v", val))
	}
	return ""
}

func getNullableString(record map[string]any, field string) *string {
	if val, ok := record[field]; ok && val != nil {
		str := strings.TrimSpace(fmt.Sprintf("%v", val))
		if str != "" && str != "<nil>" {
			return &str
		}
	}
	return nil
}

func getNullableInt(record map[string]any, field string) *int {
	if val, ok := record[field]; ok && val != nil {
		switch v := val.(type) {
		case int:
			return &v
		case int64:
			i := int(v)
			return &i
		case float64:
			i := int(v)
			return &i
		}
	}
	return nil
}

func getBoolField(record map[string]any, field string) bool {
	if val, ok := record[field]; ok && val != nil {
		switch v := val.(type) {
		case bool:
			return v
		case string:
			return v == "t" || v == "true" || v == "yes" || v == "on" || v == "1"
		}
	}
	return false
}

func getStringArray(record map[string]any, field string) []string {
	if val, ok := record[field]; ok && val != nil {
		switch v := val.(type) {
		case []string:
			return v
		case []any:
			result := make([]string, 0, len(v))
			for _, item := range v {
				if item != nil {
					result = append(result, fmt.Sprintf("%v", item))
				}
			}
			return result
		}
	}
	return nil
}
