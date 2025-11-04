package helm

import (
	"fmt"
	"time"

	flanksourceCtx "github.com/flanksource/commons-db/context"
	"github.com/flanksource/commons-test/helm"
	"github.com/flanksource/postgres/pkg/server"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type postgresChart struct {
	*helm.HelmChart
	instance *server.Postgres
	closer   func()
}

func (chart *postgresChart) GetPostgres() *server.Postgres {
	if chart.instance != nil {
		return chart.instance
	}

	pod := chart.GetPod()
	port, closer := pod.ForwardPort(5432)

	chart.instance = server.NewRemotePostgres(
		"localhost", *port,
		chart.GetValue("database", "username"),
		chart.GetPassword(),
		chart.GetValue("database", "name"),
	)
	chart.closer = closer
	return chart.instance

}

func (p *postgresChart) GetPod() *helm.Pod {
	return p.HelmChart.GetPod("app.kubernetes.io/name=postgres")
}

func (p *postgresChart) SQL(query string) string {
	password := p.GetPassword()

	output := p.GetPod().
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

	AfterEach(func() {
		if CurrentSpecReport().Failed() {
			By("Test failed, printing pod logs for debugging")
			printPodLogs(namespace, labelSelector)
		}
		if chart != nil && chart.closer != nil {
			chart.closer()
			chart.closer = nil
		}
	})

	Context("Helm Chart Operations", func() {
		It("should install with default values", func() {
			By("Installing the Helm chart using fluent API")

			Expect(chart.
				Install().Error()).NotTo(HaveOccurred())

			By("Waiting for StatefulSet to be ready")
			statefulSet := chart.GetStatefulSet(statefulSet)
			statefulSet.WaitReady()

			replicas, err := statefulSet.GetReplicas()
			Expect(err).NotTo(HaveOccurred())
			Expect(replicas).To(Equal(1))

			By("Verifying pod is running")
			pod := chart.GetPod()
			pod.WaitReady().MustSucceed()

			password := chart.GetPassword()

			By("Testing database connectivity")
			output := pod.
				Container("postgresql").
				Exec(fmt.Sprintf("PGPASSWORD=%s psql -U postgres -d postgres -c 'SELECT version();'", password)).
				Result()
			Expect(output).To(ContainSubstring("PostgreSQL"))

			server := chart.GetPostgres()
			conf, err := server.GetCurrentConf()
			Expect(err).NotTo(HaveOccurred())
			Expect(conf.ToConf()).To(HaveKeyWithValue("max_connections", "200"))
			Expect(conf.ToConf()).To(HaveKeyWithValue("shared_buffers", "1GB"))

		})

		It("should handle PostgreSQL version upgrade from 15 to 17", func() {
			By("Deleting existing release")
			chart.Delete()

			Expect(chart.Install().Error()).NotTo(HaveOccurred())

			By("Waiting for pod")
			pod := chart.GetPod()
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

		It("should auto-tune PostgreSQL settings based on resource limits", func() {
			type resourceTest struct {
				memory                     string
				cpu                        string
				expectedSharedBuffersMB    string
				expectedMaxWorkerProcesses int
				connections                int
			}

			tests := []resourceTest{
				{memory: "1Gi", cpu: "500m", expectedSharedBuffersMB: "256MB", expectedMaxWorkerProcesses: 8, connections: 64},
			}

			for _, tc := range tests {
				By(fmt.Sprintf("Testing with %s memory and %s CPU", tc.memory, tc.cpu))

				By("Installing with specific resource limits")
				Expect(chart.
					SetValue("resources.limits.memory", tc.memory).
					SetValue("resources.limits.cpu", tc.cpu).
					Install().Error()).NotTo(HaveOccurred())

				chart.GetPod().WaitReady().MustSucceed()

				server := chart.GetPostgres()
				conf, err := server.GetCurrentConf()
				Expect(err).NotTo(HaveOccurred())
				settings := conf.AsMap()
				Expect(settings["shared_buffers"].String()).To(Equal(tc.expectedSharedBuffersMB))
				Expect(settings["max_connections"].String()).To(Equal(fmt.Sprintf("%d", tc.connections)))

			}
		})

	})

})
