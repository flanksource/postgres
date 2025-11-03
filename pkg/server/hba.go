package server

import (
	"os"
	"path/filepath"

	"github.com/flanksource/clicky"
	"github.com/flanksource/postgres/pkg"
	"github.com/flanksource/postgres/pkg/generators"
)

// SetupPgHBA configures pg_hba.conf for host authentication
// Allows trust authentication from localhost and password authentication from external hosts
func (p *Postgres) SetupPgHBA(method string) error {

	var authMethod = pkg.PgAuthType(method)

	hba := generators.NewPgHBAConfigGenerator(authMethod)

	hba.AddSocketEntry("all", "postgres")
	hba.AddHostEntry("all", "all", "::1/128", pkg.AuthTrust)
	hba.AddHostEntry("all", "all", "127.0.0.1/32", pkg.AuthTrust)
	hba.AddHostEntry("all", "all", "all", authMethod)
	conf := hba.GenerateConfigFile()

	if p.DryRun {
		clicky.Infof("Generated pg_hba: \n%s", clicky.CodeBlock("properties", conf).ANSI())
		return nil
	}
	return os.WriteFile(filepath.Join(p.DataDir, "pg_hba.conf"), []byte(conf), 0644)

}
