package helm

import (
	"fmt"
	"time"

	flanksourceCtx "github.com/flanksource/commons-db/context"
	"github.com/flanksource/commons-test/helm"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type postgresChart struct {
	*helm.HelmChart
}

func (p *postgresChart) SQL(query string) string {
	password := p.GetPassword()
	pod := p.GetPod("app.kubernetes.io/name=postgres")
	output := pod.
		Container("postgresql").
		Exec(fmt.Sprintf("PGPASSWORD=%s psql -U postgres -d postgres -c \"%s\"", password, query)).
		Result()
	return output
}

func (p *postgresChart) GetPassword() string {
	secretName := p.GetValue("passwordRef", "secretName")
	Expect(secretName).NotTo(BeEmpty())
	secretKey := p.GetValue("passwordRef", "key")
	Expect(secretKey).NotTo(BeEmpty())
	password := p.Context.Lookup(namespace).WithSecretKeyRef(secretName, secretKey).MustGetString()
	Expect(password).NotTo(BeEmpty())
	return password
}

// Helm tests using fluent interface from commons
var _ = Describe("PostgreSQL Helm Chart", Ordered, func() {
	var (
		labelSelector = "app.kubernetes.io/name=postgres"
		statefulSet   = "postgres-test"
		chart         *postgresChart
	)

	BeforeEach(func() {
		clientCtx := flanksourceCtx.New().
			WithNamespace(namespace)

		_, err := clientCtx.LocalKubernetes(kubeconfig)
		Expect(err).NotTo(HaveOccurred())

		// Initialize the chart with fluent interface
		chart = &postgresChart{HelmChart: helm.NewHelmChart(clientCtx, chartPath).
			Release(releaseName).
			Namespace(namespace)}

	})

	Context("Helm Chart Operations", func() {
		It("should install with default values", func() {
			By("Installing the Helm chart using fluent API")

			Expect(chart.Install().Error()).NotTo(HaveOccurred())

			By("Waiting for StatefulSet to be ready")
			statefulSet := chart.GetStatefulSet(statefulSet)
			statefulSet.WaitReady()

			replicas, err := statefulSet.GetReplicas()
			Expect(err).NotTo(HaveOccurred())
			Expect(replicas).To(Equal(1))

			By("Verifying pod is running")
			pod := chart.GetPod(labelSelector)
			pod.WaitReady().MustSucceed()

			password := chart.GetPassword()

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

			Expect(chart.
				SetValue("resources.limits.memory", "2Gi").
				SetValue("resources.limits.cpu", "1000m").
				SetValue("conf.max_connections", "200").
				WaitFor(5 * time.Minute).
				Upgrade().Error()).NotTo(HaveOccurred())

			By("Waiting for rollout")
			ss.WaitFor(2 * time.Minute)
			newGen, err := ss.GetGeneration()
			Expect(err).NotTo(HaveOccurred())
			Expect(newGen).To(BeNumerically(">", currentGen))

			out := chart.SQL("SELECT * from pg_settings where name = 'max_connections'")
			Expect(out).To(ContainSubstring("200"))

		})

		It("should handle PostgreSQL version upgrade from 15 to 17", func() {
			By("Deleting existing release")
			chart.Delete()

			Expect(chart.Install().Error()).NotTo(HaveOccurred())

			By("Waiting for pod")
			pod := chart.GetPod(labelSelector)
			pod.WaitReady().MustSucceed()

			password := chart.GetPassword()

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

			Expect(chart.SetValue("version", "17").Upgrade().Error()).NotTo(HaveOccurred())

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

	})

})
