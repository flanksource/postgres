package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/flanksource/postgres/pkg/embedded"
	"github.com/flanksource/postgres/pkg/schemas"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--help" {
		fmt.Println("Usage: schema [postgres-version]")
		fmt.Println("Generate JSON schema from PostgreSQL CSV parameter files")
		fmt.Println("  postgres-version: PostgreSQL version to use (default: 17)")
		os.Exit(0)
	}

	version := "17"
	if len(os.Args) > 1 {
		version = os.Args[1]
	}

	fmt.Printf("Generating schema from PostgreSQL %s using describe-config...\n", version)

	// Use embedded postgres to get parameters via describe-config
	embeddedPG, err := embedded.NewEmbeddedPostgres("17.6.0")
	if err != nil {
		fmt.Printf("Error creating embedded postgres: %v\n", err)
		os.Exit(1)
	}
	defer embeddedPG.Cleanup()

	params, err := embeddedPG.DescribeConfig()
	if err != nil {
		fmt.Printf("Error getting parameters from describe-config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Loaded %d parameters from describe-config\n", len(params))

	// Add critical configuration parameters that must be included
	// These are often missing from describe-config output but are essential
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
			fmt.Printf("Added missing parameter: %s\n", criticalParam.Name)
		}
	}

	// Convert params to string format for the schema generator
	describeOutput := formatDescribeOutput(params)

	fmt.Printf("Got %d parameters, generating schema...\n", len(params))

	// Build the schema generator
	cmd := exec.Command("go", "build", "-tags", "pgtune_none", "-o", "schema/generate_schema", "./schema")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error building schema generator: %v\n", err)
		os.Exit(1)
	}

	// Write describe-config output to temporary file
	tmpFile, err := os.CreateTemp("", "postgres-describe-config-*.txt")
	if err != nil {
		fmt.Printf("Error creating temporary file: %v\n", err)
		os.Exit(1)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(describeOutput); err != nil {
		fmt.Printf("Error writing to temporary file: %v\n", err)
		os.Exit(1)
	}
	tmpFile.Close() // Close file before passing to command

	// Run the schema generator with the describe-config output file
	cmd = exec.Command("./schema/generate_schema", tmpFile.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error running schema generator: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Schema and Go struct generation complete!")
}

// formatDescribeOutput formats the parameter list as tab-separated values
func formatDescribeOutput(params []schemas.Param) string {
	var output string

	// Add header (tab-separated to match ParseDescribeConfig expected format)
	// Expected: name, context, category, vartype, boot_val, min_val, max_val, short_desc, extra_desc
	output = "name\tcontext\tcategory\tvartype\tboot_val\tmin_val\tmax_val\tshort_desc\textra_desc\n"

	// Add each parameter
	for _, param := range params {
		// Format enum values as {val1,val2,val3} or empty
		enumvals := ""
		if len(param.EnumVals) > 0 {
			enumvals = "{"
			for i, val := range param.EnumVals {
				if i > 0 {
					enumvals += ","
				}
				enumvals += val
			}
			enumvals += "}"
		}

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
