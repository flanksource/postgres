package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/flanksource/postgres/pkg"
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

	fmt.Printf("Generating schema from PostgreSQL %s CSV files...\n", version)

	// Load parameters from CSV files
	params, err := pkg.LoadParametersForVersion(version)
	if err != nil {
		fmt.Printf("Error loading parameters from CSV: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Loaded %d parameters from CSV files\n", len(params))

	// Add critical configuration parameters that must be included
	// These are often missing from the CSV files but are essential
	criticalParams := []pkg.Param{
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

	// Run the schema generator with the describe-config output
	cmd = exec.Command("./schema/generate_schema", describeOutput)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error running schema generator: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Schema and Go struct generation complete!")
}

// formatDescribeOutput formats the parameter list as pipe-separated values
func formatDescribeOutput(params []pkg.Param) string {
	var output string
	
	// Add header
	output = "name|setting|unit|category|short_desc|extra_desc|context|vartype|source|min_val|max_val|enumvals|boot_val|reset_val|sourcefile|sourceline|pending_restart\n"
	
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

		// Format pending restart as t/f (not available in pkg.Param, default to f)
		pendingRestart := "f"
		
		// Handle empty values - pkg.Param fields are different
		setting := "\\N" // Not available in pkg.Param
		unit := param.Unit
		if unit == "" {
			unit = "\\N"
		}
		category := param.Category
		if category == "" {
			category = "\\N"
		}
		shortDesc := param.ShortDesc
		if shortDesc == "" {
			shortDesc = "\\N"
		}
		extraDesc := param.ExtraDesc
		if extraDesc == "" {
			extraDesc = "\\N"
		}
		context := param.Context
		if context == "" {
			context = "\\N"
		}
		vartype := param.VarType
		if vartype == "" {
			vartype = "\\N"
		}
		source := "\\N" // Not available in pkg.Param
		minVal := fmt.Sprintf("%.0f", param.MinVal)
		if minVal == "0" {
			minVal = "\\N"
		}
		maxVal := fmt.Sprintf("%.0f", param.MaxVal)
		if maxVal == "0" {
			maxVal = "\\N"
		}
		if enumvals == "" {
			enumvals = "\\N"
		}
		bootVal := param.BootVal
		if bootVal == "" {
			bootVal = "\\N"
		}
		resetVal := "\\N" // Not available in pkg.Param
		sourceFile := "\\N" // Not available in pkg.Param
		sourceLine := "\\N" // Not available in pkg.Param
		
		line := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s\n",
			param.Name, setting, unit, category, shortDesc, extraDesc,
			context, vartype, source, minVal, maxVal, enumvals,
			bootVal, resetVal, sourceFile, sourceLine, pendingRestart)
		
		output += line
	}
	
	return output
}