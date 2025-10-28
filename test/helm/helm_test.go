package helm

import (
	"fmt"
	"time"

	flanksourceCtx "github.com/flanksource/commons-db/context"
	"github.com/flanksource/commons-test/helm"
	"github.com/flanksource/commons/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Helm tests using fluent interface from commons
var _ = Describe("PostgreSQL Helm Chart", Ordered, func() {
	var (
		labelSelector  = "app.kubernetes.io/name=postgres"
		statefulSet    = "postgres-test"
		passwordSecret = "postgres-test-postgres-postgres"
		chart          *helm.HelmChart
		watcher        *PodWatcher
	)

	BeforeEach(func() {
		clientCtx := flanksourceCtx.New().
			WithNamespace(namespace)

		_, err := clientCtx.LocalKubernetes(kubeconfig)
		Expect(err).NotTo(HaveOccurred())

		// Initialize the chart with fluent interface
		chart = helm.NewHelmChart(clientCtx, chartPath).
			Release(releaseName).
			Namespace(namespace)

		// Start pod watcher for real-time log streaming and failure detection

		watcher, err = NewPodWatcher(namespace, kubeconfig, logger)
		Expect(err).NotTo(HaveOccurred())

		err = watcher.Start()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		// Stop pod watcher
		if watcher != nil {
			watcher.Stop()
		}
	})

	Context("Helm Chart Operations", func() {
		It("should install with default values", func() {
			By("Installing the Helm chart using fluent API")

			// Run helm install in background while watcher monitors
			installDone := make(chan error, 1)
			go func() {
				chart.
					WaitFor(5 * time.Minute).
					Install()
				installDone <- chart.Error()
			}()

			// Wait for either install completion or pod failure
			By("Monitoring pods for failures")
			select {
			case err := <-watcher.FailureChan():
				Fail(fmt.Sprintf("Pod failure detected: %v", err))
			case err := <-installDone:
				Expect(err).NotTo(HaveOccurred())
			case <-time.After(6 * time.Minute):
				Fail("Helm install timed out")
			}

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
			secretName := chart.GetValue("passwordRef", "secretName")
			secretKey := chart.GetValue("passwordRef", "key")
			By(fmt.Sprintf("Getting password from secret %s/%s", secretName, secretKey))
			password := chart.Context.Lookup(namespace).WithSecretKeyRef(secretName, secretKey).MustGetString()

			Expect(password).NotTo(BeEmpty())

			By("Testing database connectivity")
			output := pod.
				Container("postgresql").
				Exec(fmt.Sprintf("PGPASSWORD=%s psql -U postgres -d postgres -c 'SELECT version();'", password)).
				Result()
			Expect(output).To(ContainSubstring("PostgreSQL"))
		})

		PIt("should upgrade with new configuration", func() {
			By("Getting current generation")
			ss := chart.GetStatefulSet(statefulSet)
			currentGen, err := ss.GetGeneration()
			Expect(err).NotTo(HaveOccurred())

			By("Upgrading with new values")
			upgradeDone := make(chan error, 1)
			go func() {
				chart.
					SetValue("resources.limits.memory", "2Gi").
					SetValue("resources.limits.cpu", "1000m").
					SetValue("conf.max_connections", "200").
					WaitFor(5 * time.Minute).
					Upgrade()
				upgradeDone <- chart.Error()
			}()

			// Monitor for failures during upgrade
			select {
			case err := <-watcher.FailureChan():
				Fail(fmt.Sprintf("Pod failure during upgrade: %v", err))
			case err := <-upgradeDone:
				Expect(err).NotTo(HaveOccurred())
			case <-time.After(6 * time.Minute):
				Fail("Helm upgrade timed out")
			}

			By("Waiting for rollout")
			ss.WaitFor(2 * time.Minute)
			newGen, err := ss.GetGeneration()
			Expect(err).NotTo(HaveOccurred())
			Expect(newGen).To(BeNumerically(">", currentGen))

			By("Verifying configuration was updated")
			configMap := chart.GetConfigMap(releaseName + "-postgres-config")
			config, err := configMap.Get("postgresql.conf")
			Expect(err).NotTo(HaveOccurred())
			Expect(config).To(ContainSubstring("max_connections"))
		})

		PIt("should handle PostgreSQL version upgrade from 15 to 17", func() {
			By("Deleting existing release")
			chart.Delete()

			By("Installing PostgreSQL 15")
			installDone := make(chan error, 1)
			go func() {
				chart.
					Values(map[string]interface{}{
						"image": map[string]interface{}{
							"tag": "15",
						},
					}).
					WaitFor(5 * time.Minute).
					Install()
				installDone <- chart.Error()
			}()

			// Monitor for failures
			select {
			case err := <-watcher.FailureChan():
				Fail(fmt.Sprintf("Pod failure during PG15 install: %v", err))
			case err := <-installDone:
				Expect(err).NotTo(HaveOccurred())
			case <-time.After(6 * time.Minute):
				Fail("PostgreSQL 15 install timed out")
			}

			By("Waiting for pod")
			pod := chart.GetPod(labelSelector)
			pod.WaitReady().MustSucceed()

			By("Getting password")
			secret := chart.GetSecret(passwordSecret)
			password, err := secret.Get("postgres-password")
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
				Container("postgresql").
				Exec(fmt.Sprintf("PGPASSWORD=%s psql -U postgres -d postgres -c 'SELECT COUNT(*) FROM test_upgrade;'", password)).
				Result()
			Expect(output).To(ContainSubstring("3"))

			By("Upgrading to PostgreSQL 17")
			upgradeDone := make(chan error, 1)
			go func() {
				chart.
					Values(map[string]interface{}{
						"image": map[string]interface{}{
							"tag": "17",
						},
						"passwordRef": map[string]interface{}{
							"secretName": passwordSecret,
							"key":        "postgres-password",
							"create":     false,
						},
					}).
					WaitFor(10 * time.Minute).
					Upgrade()
				upgradeDone <- chart.Error()
			}()

			// Monitor for failures during upgrade to PG17
			select {
			case err := <-watcher.FailureChan():
				Fail(fmt.Sprintf("Pod failure during PG17 upgrade: %v", err))
			case err := <-upgradeDone:
				Expect(err).NotTo(HaveOccurred())
			case <-time.After(11 * time.Minute):
				Fail("PostgreSQL 17 upgrade timed out")
			}

			By("Waiting for upgrade completion")
			ss := chart.GetStatefulSet(statefulSet)
			ss.WaitFor(5 * time.Minute)

			By("Waiting for pod after upgrade")
			pod.WaitReady().MustSucceed()

			By("Verifying data persisted")
			output = pod.
				Container("postgresql").
				Exec(fmt.Sprintf("PGPASSWORD=%s psql -U postgres -d postgres -c 'SELECT COUNT(*) FROM test_upgrade;'", password)).
				Result()
			Expect(output).To(ContainSubstring("3"))

			By("Checking PostgreSQL version")
			output = pod.
				Container("postgresql").
				Exec(fmt.Sprintf("PGPASSWORD=%s psql -U postgres -d postgres -c 'SELECT version();'", password)).
				Result()
			GinkgoWriter.Printf("PostgreSQL version after upgrade: %s\n", output)
		})

		PIt("should support complex chained operations", func() {
			By("Complex chained operation")

			// This demonstrates the full power of the fluent interface
			chart.
				SetValue("conf.max_connections", "150").
				SetValue("resources.limits.memory", "3Gi").
				SetValue("persistence.size", "15Gi").
				Values(map[string]interface{}{
					"conf": map[string]interface{}{
						"log_statement":              "all",
						"log_min_duration_statement": "100",
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
				Container("postgresql").
				Exec("echo 'First command'").
				MustSucceed().
				Container("postgresql").
				Exec("echo 'Second command'").
				MustSucceed()

			// Work with ConfigMaps
			cm := chart.GetConfigMap(releaseName + "-postgres-config")
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

	PContext("Namespace Management", func() {
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
