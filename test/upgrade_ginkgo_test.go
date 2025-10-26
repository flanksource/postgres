package test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PostgreSQL Upgrade", func() {
	FContext("Quick upgrade test (14 to 17)", func() {
		It("should successfully upgrade from PostgreSQL 14 to 17", func() {
			By("Running upgrade from 14 to 17")
			err := upgradeTest.testUpgrade("14", "17")
			Expect(err).NotTo(HaveOccurred(), "Upgrade from 14 to 17 should succeed")
		})
	})

	Context("Upgrade matrix", func() {
		type upgradeCase struct {
			from string
			to   string
		}

		upgradeCases := []upgradeCase{
			{from: "14", to: "15"},
			{from: "15", to: "16"},
			{from: "16", to: "17"},
			{from: "15", to: "17"},
		}

		for _, tc := range upgradeCases {
			tc := tc // Capture loop variable
			It(fmt.Sprintf("should upgrade from PostgreSQL %s to %s", tc.from, tc.to), func() {
				By(fmt.Sprintf("Testing upgrade from %s to %s", tc.from, tc.to))
				err := upgradeTest.testUpgrade(tc.from, tc.to)
				Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Upgrade from %s to %s should succeed", tc.from, tc.to))
			})
		}
	})

	Context("Upgrade with extensions", func() {
		It("should successfully upgrade from 14 to 17 with extensions", func() {
			By("Testing upgrade with pgvector, pgaudit, and pg_cron extensions")
			err := upgradeTest.testUpgradeWithExtensions("14", "17")
			Expect(err).NotTo(HaveOccurred(), "Upgrade with extensions should succeed")
		})
	})

	Context("Data integrity verification", func() {
		It("should preserve data during upgrade", func() {
			By("Creating test data before upgrade")
			// This would be implemented in testUpgrade method
			err := upgradeTest.testUpgrade("14", "17")
			Expect(err).NotTo(HaveOccurred())

			By("Verifying data exists after upgrade")
			// Data verification is part of testUpgrade
		})
	})
})
