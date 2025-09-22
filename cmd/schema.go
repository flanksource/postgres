package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/flanksource/postgres/pkg/embedded"
	"github.com/flanksource/postgres/pkg/schemas"
	"github.com/spf13/cobra"
)

// createSchemaCommands creates the schema command group
func createSchemaCommands() *cobra.Command {
	schemaCmd := &cobra.Command{
		Use:   "schema",
		Short: "PostgreSQL schema operations",
		Long:  "Commands for generating and managing JSON schemas from PostgreSQL parameters",
	}

	schemaCmd.AddCommand(
		createSchemaGenerateCommand(),
		createSchemaValidateCommand(),
		createSchemaReportCommand(),
	)

	return schemaCmd
}

// createSchemaGenerateCommand creates the schema generate command
func createSchemaGenerateCommand() *cobra.Command {
	generateCmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate JSON schema from PostgreSQL parameters",
		Long:  "Generate JSON schema from PostgreSQL CSV parameter files using describe-config",
		RunE:  runSchemaGenerate,
	}

	generateCmd.Flags().String("postgres-version", "17", "PostgreSQL version to use")
	generateCmd.Flags().String("output", "", "Output file for generated schema (default: stdout)")

	return generateCmd
}

// createSchemaValidateCommand creates the schema validate command
func createSchemaValidateCommand() *cobra.Command {
	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate existing schema against PostgreSQL",
		Long:  "Validate existing schema against PostgreSQL parameters",
		RunE:  runSchemaValidate,
	}

	validateCmd.Flags().String("postgres-version", "17", "PostgreSQL version to validate against")
	validateCmd.Flags().String("schema-file", "", "Schema file to validate (required)")
	validateCmd.MarkFlagRequired("schema-file")

	return validateCmd
}

// createSchemaReportCommand creates the schema report command
func createSchemaReportCommand() *cobra.Command {
	reportCmd := &cobra.Command{
		Use:   "report",
		Short: "Generate parameter report",
		Long:  "Generate a parameter report from PostgreSQL",
		RunE:  runSchemaReport,
	}

	reportCmd.Flags().String("postgres-version", "17", "PostgreSQL version to use")
	reportCmd.Flags().String("output", "", "Output file for report (default: stdout)")
	reportCmd.Flags().String("format", "markdown", "Report format (markdown, json, text)")

	return reportCmd
}

// runSchemaGenerate handles the schema generate command
func runSchemaGenerate(cmd *cobra.Command, args []string) error {
	version, _ := cmd.Flags().GetString("postgres-version")
	outputFile, _ := cmd.Flags().GetString("output")

	if verbose {
		fmt.Printf("Generating schema from PostgreSQL %s using describe-config...\n", version)
	}

	// Use embedded postgres to get parameters via describe-config
	embeddedPG, err := embedded.NewEmbeddedPostgres("17.6.0")
	if err != nil {
		return fmt.Errorf("error creating embedded postgres: %w", err)
	}
	defer embeddedPG.Cleanup()

	params, err := embeddedPG.DescribeConfig()
	if err != nil {
		return fmt.Errorf("error getting parameters from describe-config: %w", err)
	}

	if verbose {
		fmt.Printf("Loaded %d parameters from describe-config\n", len(params))
	}

	// Add critical configuration parameters that must be included
	criticalParams := []schemas.Param{
		{
			Name:      "listen_addresses",
			Context:   "postmaster",
			Category:  "Connections and Authentication / Connection Settings",
			VarType:   "string",
			BootVal:   "localhost",
			MinVal:    0,
			MaxVal:    0,
			ShortDesc: "Sets the host name or IP address(es) to listen to.",
			ExtraDesc: "",
			VarClass:  "configuration",
		},
		{
			Name:      "port",
			Context:   "postmaster",
			Category:  "Connections and Authentication / Connection Settings",
			VarType:   "integer",
			BootVal:   "5432",
			MinVal:    1,
			MaxVal:    65535,
			ShortDesc: "Sets the server's port number.",
			ExtraDesc: "",
			VarClass:  "configuration",
		},
	}

	// Check if these parameters are already present
	paramNames := make(map[string]bool)
	for _, param := range params {
		paramNames[param.Name] = true
	}

	// Add missing critical parameters
	for _, criticalParam := range criticalParams {
		if !paramNames[criticalParam.Name] {
			params = append(params, criticalParam)
			if verbose {
				fmt.Printf("Added missing parameter: %s\n", criticalParam.Name)
			}
		}
	}

	// Convert params to string format for the schema generator
	describeOutput := formatDescribeOutput(params)

	if verbose {
		fmt.Printf("Got %d parameters, generating schema...\n", len(params))
	}

	// Build the schema generator
	buildCmd := exec.Command("go", "build", "-tags", "pgtune_none", "-o", "schema/generate_schema", "./schema")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("error building schema generator: %w", err)
	}

	// Write describe-config output to temporary file
	tmpFile, err := os.CreateTemp("", "postgres-describe-config-*.txt")
	if err != nil {
		return fmt.Errorf("error creating temporary file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(describeOutput); err != nil {
		return fmt.Errorf("error writing to temporary file: %w", err)
	}
	tmpFile.Close() // Close file before passing to command

	// Run the schema generator with the describe-config output file
	var outputCmd *exec.Cmd
	if outputFile != "" {
		outputCmd = exec.Command("./schema/generate_schema", tmpFile.Name())
		outputCmd.Stderr = os.Stderr

		output, err := outputCmd.Output()
		if err != nil {
			return fmt.Errorf("error running schema generator: %w", err)
		}

		if err := os.WriteFile(outputFile, output, 0644); err != nil {
			return fmt.Errorf("error writing output file: %w", err)
		}

		fmt.Printf("✅ Schema generated and saved to: %s\n", outputFile)
	} else {
		outputCmd = exec.Command("./schema/generate_schema", tmpFile.Name())
		outputCmd.Stdout = os.Stdout
		outputCmd.Stderr = os.Stderr
		if err := outputCmd.Run(); err != nil {
			return fmt.Errorf("error running schema generator: %w", err)
		}
	}

	return nil
}

// runSchemaValidate handles the schema validate command
func runSchemaValidate(cmd *cobra.Command, args []string) error {
	version, _ := cmd.Flags().GetString("postgres-version")
	schemaFile, _ := cmd.Flags().GetString("schema-file")

	if verbose {
		fmt.Printf("Validating schema file %s against PostgreSQL %s\n", schemaFile, version)
	}

	// Check if schema file exists
	if _, err := os.Stat(schemaFile); os.IsNotExist(err) {
		return fmt.Errorf("schema file does not exist: %s", schemaFile)
	}

	// TODO: Implement actual schema validation logic
	// This would involve loading the schema and comparing it against PostgreSQL parameters
	fmt.Printf("Schema validation for %s would be implemented here\n", schemaFile)
	fmt.Println("✅ Schema validation placeholder completed")

	return nil
}

// runSchemaReport handles the schema report command
func runSchemaReport(cmd *cobra.Command, args []string) error {
	version, _ := cmd.Flags().GetString("postgres-version")
	outputFile, _ := cmd.Flags().GetString("output")
	format, _ := cmd.Flags().GetString("format")

	if verbose {
		fmt.Printf("Generating parameter report for PostgreSQL %s in %s format\n", version, format)
	}

	// TODO: Implement actual report generation logic
	// This would involve gathering PostgreSQL parameters and formatting them as a report
	reportContent := generateParameterReport(version, format)

	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(reportContent), 0644); err != nil {
			return fmt.Errorf("error writing report file: %w", err)
		}
		fmt.Printf("✅ Parameter report generated and saved to: %s\n", outputFile)
	} else {
		fmt.Print(reportContent)
	}

	return nil
}

// formatDescribeOutput formats the parameter list as tab-separated values
func formatDescribeOutput(params []schemas.Param) string {
	var output string

	// Add header (tab-separated to match ParseDescribeConfig expected format)
	output = "name\tcontext\tcategory\tvartype\tboot_val\tmin_val\tmax_val\tshort_desc\textra_desc\n"

	// Add each parameter
	for _, param := range params {
		// Handle empty values properly
		name := param.Name
		context := param.Context
		if context == "" {
			context = "\\N"
		}
		category := param.Category
		if category == "" {
			category = "\\N"
		}
		vartype := strings.ToUpper(param.VarType) // Convert to uppercase to match postgres format
		if vartype == "" {
			vartype = "\\N"
		}
		bootVal := param.BootVal
		if bootVal == "" {
			bootVal = "\\N"
		}
		minVal := fmt.Sprintf("%.0f", param.MinVal)
		if minVal == "0" {
			minVal = ""
		}
		maxVal := fmt.Sprintf("%.0f", param.MaxVal)
		if maxVal == "0" {
			maxVal = ""
		}
		shortDesc := param.ShortDesc
		if shortDesc == "" {
			shortDesc = "\\N"
		}
		extraDesc := param.ExtraDesc
		if extraDesc == "" {
			extraDesc = "\\N"
		}

		// Format: name, context, category, vartype, boot_val, min_val, max_val, short_desc, extra_desc
		line := fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			name, context, category, vartype, bootVal, minVal, maxVal, shortDesc, extraDesc)

		output += line
	}

	return output
}

// generateParameterReport generates a parameter report (placeholder implementation)
func generateParameterReport(version, format string) string {
	switch format {
	case "markdown":
		return fmt.Sprintf(`# PostgreSQL %s Parameter Report

This is a placeholder parameter report.

## Configuration Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| max_connections | integer | 100 | Maximum number of concurrent connections |
| shared_buffers | string | 128MB | Amount of memory for shared buffers |
| work_mem | string | 4MB | Amount of memory for work operations |

Generated on: %s
`, version, "now")

	case "json":
		return fmt.Sprintf(`{
  "postgresql_version": "%s",
  "report_type": "parameters",
  "generated_at": "now",
  "parameters": [
    {
      "name": "max_connections",
      "type": "integer",
      "default": "100",
      "description": "Maximum number of concurrent connections"
    },
    {
      "name": "shared_buffers",
      "type": "string",
      "default": "128MB",
      "description": "Amount of memory for shared buffers"
    }
  ]
}`, version)

	case "text":
		return fmt.Sprintf(`PostgreSQL %s Parameter Report
==================================

This is a placeholder parameter report.

Configuration Parameters:
- max_connections (integer): 100 - Maximum number of concurrent connections
- shared_buffers (string): 128MB - Amount of memory for shared buffers
- work_mem (string): 4MB - Amount of memory for work operations

Generated on: now
`, version)

	default:
		return fmt.Sprintf("Unsupported format: %s", format)
	}
}