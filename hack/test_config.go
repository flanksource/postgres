package main

import (
	"fmt"
	"os"
	"path/filepath"
	"github.com/flanksource/postgres/pkg"
	"gopkg.in/yaml.v3"
)

func main() {
	// Create test config
	testConfig := map[string]interface{}{
		"postgres": map[string]interface{}{
			"port":            5432,
			"max_connections": 100,
			"shared_buffers":  "128MB",
		},
	}

	tempDir, _ := os.MkdirTemp("", "test")
	defer os.RemoveAll(tempDir)

	yamlData, _ := yaml.Marshal(testConfig)
	configFile := filepath.Join(tempDir, "test.yaml")
	os.WriteFile(configFile, yamlData, 0644)

	// Try to load it
	conf, err := pkg.LoadConfigWithValidation(configFile, "schema/pgconfig-schema.json")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Print what we got
	if conf.Postgres != nil {
		fmt.Printf("Loaded config:\n")
		fmt.Printf("  Port: %v\n", conf.Postgres.Port)
		fmt.Printf("  MaxConnections: %v\n", conf.Postgres.MaxConnections)
		fmt.Printf("  SharedBuffers: %v\n", conf.Postgres.SharedBuffers)
		fmt.Printf("  WalBuffers: %v\n", conf.Postgres.WalBuffers)
		fmt.Printf("  WorkMem: %v\n", conf.Postgres.WorkMem)
	}
}
