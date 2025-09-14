package main

import (
	"fmt"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"os"
	"path/filepath"
	ym "gopkg.in/yaml.v3"
)

func main() {
	// Create test config
	testConfig := map[string]interface{}{
		"postgres": map[string]interface{}{
			"work_mem": "4MB",
			"wal_buffers": "16MB",
		},
	}

	tempDir, _ := os.MkdirTemp("", "test")
	defer os.RemoveAll(tempDir)

	yamlData, _ := ym.Marshal(testConfig)
	configFile := filepath.Join(tempDir, "test.yaml")
	os.WriteFile(configFile, yamlData, 0644)

	// Load with koanf
	k := koanf.New(".")
	k.Load(file.Provider(configFile), yaml.Parser())

	// Check what keys exist
	fmt.Println("Does postgres.work_mem exist?", k.Exists("postgres.work_mem"))
	fmt.Println("Value of postgres.work_mem:", k.Get("postgres.work_mem"))
	
	// Now try to set it if it doesn't exist
	if !k.Exists("postgres.work_mem") {
		k.Set("postgres.work_mem", "SHOULD_NOT_SET")
		fmt.Println("Set default value")
	} else {
		fmt.Println("Key exists, not setting default")
	}
	
	fmt.Println("Final value:", k.Get("postgres.work_mem"))
}
