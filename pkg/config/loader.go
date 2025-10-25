package config

import (
	"encoding/json"
	"fmt"
	"os"

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
