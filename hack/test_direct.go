package main

import (
	"fmt"
	"github.com/flanksource/postgres/pkg"
)

func main() {
	// Use the actual test file
	conf, err := pkg.LoadConfigWithValidation("test-config/fixtures/valid-complete.yaml", "schema/pgconfig-schema.json")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Print what we got
	if conf.Postgres != nil {
		fmt.Printf("Loaded config:\n")
		if conf.Postgres.SharedBuffers != nil {
			fmt.Printf("  SharedBuffers: %v\n", *conf.Postgres.SharedBuffers)
		}
		if conf.Postgres.WalBuffers != nil {
			fmt.Printf("  WalBuffers: %v\n", *conf.Postgres.WalBuffers)
		}
	}
}
