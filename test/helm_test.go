package test

import (
	"fmt"
	"time"

	"github.com/flanksource/commons/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Helm tests using fluent interface from commons
var _ = Describe("PostgreSQL Upgrade Helm Chart", Ordered, func() {
	var (
		labelSelector  = "app.kubernetes.io/name=postgres-upgrade"
		statefulSet    = "postgres-test-postgres-upgrade"
		passwordSecret = "postgres-test-password"
		chart          *test.HelmChart
	)

	BeforeEach(func() {
		// Initialize the chart with fluent interface
		chart = test.NewHelmChart(chartPath).
			Release(releaseName).
			Namespace(namespace)
	})

	Context("Helm Chart Operations", func() {
		It("should install with default values", func() {
			By("Installing the Helm chart using fluent API")

			// Install with password secret and wait
			chart.
				WithPassword(passwordSecret).
				WaitFor(5 * time.Minute).
				Install().
				MustSucceed()

			By("Waiting for StatefulSet to be ready")
			statefulSet := chart.GetStatefulSet(statefulSet)
			statefulSet.WaitReady()

			replicas, err := statefulSet.GetReplicas()
			Expect(err).NotTo(HaveOccurred())
			Expect(replicas).To(Equal(1))

			By("Verifying pod is running")
			pod := chart.GetPod(labelSelector)
			pod.WaitReady().MustSucceed()

			By("Getting password from secret")
			secret := chart.GetSecret(passwordSecret)
			password, err := secret.Get("password")
			Expect(err).NotTo(HaveOccurred())
			Expect(password).NotTo(BeEmpty())

			By("Testing database connectivity")
			output := pod.
				Container("postgresql").
				Exec(fmt.Sprintf("PGPASSWORD=%s psql -U postgres -d postgres -c 'SELECT version();'", password)).
				Result()
			Expect(output).To(ContainSubstring("PostgreSQL"))
		})

		It("should upgrade with new configuration", func() {
			By("Getting current generation")
			ss := chart.GetStatefulSet(statefulSet)
			currentGen, err := ss.GetGeneration()
			Expect(err).NotTo(HaveOccurred())

			By("Upgrading with new values")
			chart.
				SetValue("resources.limits.memory", "2Gi").
				SetValue("resources.limits.cpu", "1000m").
				SetValue("database.config.max_connections", "200").
				WaitFor(5 * time.Minute).
				Upgrade().
				MustSucceed()

			By("Waiting for rollout")
			ss.WaitFor(2 * time.Minute)
			newGen, err := ss.GetGeneration()
			Expect(err).NotTo(HaveOccurred())
			Expect(newGen).To(BeNumerically(">", currentGen))

			By("Verifying configuration was updated")
			configMap := chart.GetConfigMap(releaseName + "-postgres-upgrade-config")
			config, err := configMap.Get("postgresql.conf")
			Expect(err).NotTo(HaveOccurred())
			Expect(config).To(ContainSubstring("max_connections = 200"))
		})

		It("should handle PostgreSQL version upgrade from 15 to 17", func() {
			By("Deleting existing release")
			chart.Delete()

			By("Installing PostgreSQL 15")
			chart.
				WithPassword(passwordSecret).
				Values(map[string]interface{}{
					"postgresql": map[string]interface{}{
						"image": map[string]interface{}{
							"tag": "15",
						},
					},
					"database": map[string]interface{}{
						"version":     "15",
						"autoUpgrade": false,
					},
					"upgradeContainer": map[string]interface{}{
						"enabled": false,
					},
				}).
				WaitFor(5 * time.Minute).
				Install().
				MustSucceed()

			By("Waiting for pod")
			pod := chart.GetPod(labelSelector)
			pod.WaitReady().MustSucceed()

			By("Getting password")
			secret := chart.GetSecret(passwordSecret)
			password, err := secret.Get("password")
			Expect(err).NotTo(HaveOccurred())

			By("Creating test data")
			pod.Container("postgresql").
				Exec(fmt.Sprintf(`PGPASSWORD=%s psql -U postgres -d postgres -c "
					CREATE TABLE test_upgrade (id serial PRIMARY KEY, data text);
					INSERT INTO test_upgrade (data) VALUES ('test1'), ('test2'), ('test3');
				"`, password)).
				MustSucceed()

			By("Verifying data")
			output := pod.
				Exec(fmt.Sprintf("PGPASSWORD=%s psql -U postgres -d postgres -c 'SELECT COUNT(*) FROM test_upgrade;'", password)).
				Result()
			Expect(output).To(ContainSubstring("3"))

			By("Upgrading to PostgreSQL 17")
			chart.
				Values(map[string]interface{}{
					"postgresql": map[string]interface{}{
						"image": map[string]interface{}{
							"tag": "17",
						},
					},
					"database": map[string]interface{}{
						"version":        "17",
						"autoUpgrade":    true,
						"existingSecret": passwordSecret,
						"secretKey":      "password",
					},
					"upgradeContainer": map[string]interface{}{
						"enabled": true,
						"image": map[string]interface{}{
							"repository": "flanksource/docker-postgres-upgrade-upgrade",
							"tag":        "latest",
						},
					},
				}).
				WaitFor(10 * time.Minute).
				Upgrade().
				MustSucceed()

			By("Waiting for upgrade completion")
			ss := chart.GetStatefulSet(statefulSet)
			ss.WaitFor(5 * time.Minute)

			By("Waiting for pod after upgrade")
			pod.WaitReady().MustSucceed()

			By("Verifying data persisted")
			output = pod.
				Exec(fmt.Sprintf("PGPASSWORD=%s psql -U postgres -d postgres -c 'SELECT COUNT(*) FROM test_upgrade;'", password)).
				Result()
			Expect(output).To(ContainSubstring("3"))

			By("Checking PostgreSQL version")
			output = pod.
				Exec(fmt.Sprintf("PGPASSWORD=%s psql -U postgres -d postgres -c 'SELECT version();'", password)).
				Result()
			GinkgoWriter.Printf("PostgreSQL version after upgrade: %s\n", output)
		})

		It("should support complex chained operations", func() {
			By("Complex chained operation")

			// This demonstrates the full power of the fluent interface
			chart.
				WithPassword("admin-secret").
				SetValue("database.config.max_connections", "150").
				SetValue("resources.limits.memory", "3Gi").
				SetValue("persistence.size", "15Gi").
				Values(map[string]interface{}{
					"database": map[string]interface{}{
						"config": map[string]interface{}{
							"custom": map[string]interface{}{
								"log_statement":              "all",
								"log_min_duration_statement": "100",
							},
						},
					},
				}).
				WaitFor(7 * time.Minute).
				Upgrade()

			// Check for errors but don't panic
			if err := chart.Error(); err != nil {
				GinkgoWriter.Printf("Upgrade encountered error: %v\n", err)
				// Could inspect the result for more details
				result := chart.Result()
				GinkgoWriter.Printf("Command output: %s\n", result.String())
			}

			// Get multiple resources in a chain
			pod := chart.GetPod(labelSelector)
			pod.WaitReady()

			// Get logs with line limit
			logs := pod.GetLogs(50)
			GinkgoWriter.Printf("Last 50 lines of logs:\n%s\n", logs)

			// Check pod status
			status, err := pod.Status()
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Equal("Running"))

			// Exec multiple commands
			pod.
				Exec("echo 'First command'").
				MustSucceed().
				Exec("echo 'Second command'").
				MustSucceed()

			// Work with ConfigMaps
			cm := chart.GetConfigMap(releaseName + "-postgres-upgrade-config")
			config, err := cm.Get("postgresql.conf")
			Expect(err).NotTo(HaveOccurred())
			Expect(config).To(ContainSubstring("max_connections = 150"))

			// Work with PVCs
			pvc := chart.GetPVC("data-" + statefulSet + "-0")
			pvcStatus, err := pvc.Status()
			Expect(err).NotTo(HaveOccurred())
			pvcStatusMap := pvcStatus["status"].(map[string]interface{})
			Expect(pvcStatusMap["phase"]).To(Equal("Bound"))
		})
	})

	Context("Namespace Management", func() {
		It("should manage namespaces", func() {
			testNs := test.NewNamespace("test-fluent-ns")

			By("Creating namespace")
			testNs.Create().MustSucceed()

			By("Installing chart in new namespace")
			testChart := test.NewHelmChart(chartPath).
				Release("test-release").
				Namespace("test-fluent-ns").
				WithPassword("test-secret").
				Install()

			if testChart.Error() == nil {
				By("Cleaning up")
				testChart.Delete()
			}

			By("Deleting namespace")
			testNs.Delete().MustSucceed()
		})
	})
})
