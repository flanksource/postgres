package test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	upgradeTest *PostgresUpgradeTest
	config      *PostgresUpgradeConfig
)

func TestUpgrade(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PostgreSQL Upgrade Suite")
}

var _ = BeforeSuite(func() {
	config = &PostgresUpgradeConfig{
		ImageName:      "flanksource/postgres:latest",
		Registry:       "ghcr.io",
		ImageBase:      "flanksource/postgres",
		SourceVersions: []string{"14", "15", "16"},
		TargetVersion:  "17",
		TestUser:       "testuser",
		TestPassword:   "testpass",
		TestDatabase:   "testdb",
		Extensions:     []string{"pgvector", "pgaudit", "pg_cron"},
	}

	upgradeTest = NewPostgresUpgradeTest(config)

	By("Building PostgreSQL upgrade image")
	err := upgradeTest.buildUpgradeImage()
	Expect(err).NotTo(HaveOccurred(), "Failed to build upgrade image")
})

var _ = AfterSuite(func() {
	By("Cleaning up test volumes")
	upgradeTest.cleanup()
})
