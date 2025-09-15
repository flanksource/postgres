package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/flanksource/postgres/pkg/generators"
)

func generateSchema(cmd interface{}, args []string) error {
	// Parse flags
	flagSet := flag.NewFlagSet("generate", flag.ExitOnError)
	schemaVersion := flagSet.String("version", "16.1.0", "PostgreSQL version to use")
	schemaOutputFile := flagSet.String("output", "schema/pgconfig-schema-generated.json", "Output schema file")
	flagSet.Parse(args)

	fmt.Printf("Generating schema for PostgreSQL %s...\n", *schemaVersion)

	generator, err := generators.NewSchemaGenerator(*schemaVersion)
	if err != nil {
		return fmt.Errorf("failed to create schema generator: %w", err)
	}

	// Ensure output directory exists
	outputDir := filepath.Dir(*schemaOutputFile)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if err := generator.WriteSchemaFile(*schemaOutputFile); err != nil {
		return fmt.Errorf("failed to write schema file: %w", err)
	}

	fmt.Printf("Schema generated successfully: %s\n", *schemaOutputFile)
	return nil
}

func validateSchema(cmd interface{}, args []string) error {
	// Parse flags
	flagSet := flag.NewFlagSet("validate", flag.ExitOnError)
	schemaVersion := flagSet.String("version", "16.1.0", "PostgreSQL version to use")
	flagSet.Parse(args)

	fmt.Printf("Validating existing schema against PostgreSQL %s...\n", *schemaVersion)

	generator, err := generators.NewSchemaGenerator(*schemaVersion)
	if err != nil {
		return fmt.Errorf("failed to create schema generator: %w", err)
	}

	issues, err := generator.ValidateExistingSchema()
	if err != nil {
		return fmt.Errorf("failed to validate schema: %w", err)
	}

	if len(issues) == 0 {
		fmt.Println("✓ Schema validation passed - no issues found")
		return nil
	}

	fmt.Printf("Found %d issues:\n\n", len(issues))
	for _, issue := range issues {
		fmt.Printf("- %s\n", issue)
	}

	return nil
}

func generateReport(cmd interface{}, args []string) error {
	// Parse flags
	flagSet := flag.NewFlagSet("report", flag.ExitOnError)
	schemaVersion := flagSet.String("version", "16.1.0", "PostgreSQL version to use")
	schemaOutputFile := flagSet.String("output", "docs/parameters.md", "Output report file")
	flagSet.Parse(args)

	fmt.Printf("Generating parameter report for PostgreSQL %s...\n", *schemaVersion)

	generator, err := generators.NewSchemaGenerator(*schemaVersion)
	if err != nil {
		return fmt.Errorf("failed to create schema generator: %w", err)
	}

	report, err := generator.GenerateParameterReport()
	if err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	// Ensure output directory exists
	outputDir := filepath.Dir(*schemaOutputFile)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if err := os.WriteFile(*schemaOutputFile, []byte(report), 0644); err != nil {
		return fmt.Errorf("failed to write report file: %w", err)
	}

	fmt.Printf("Report generated successfully: %s\n", *schemaOutputFile)
	return nil
}
