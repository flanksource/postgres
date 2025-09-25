package test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	kubeconfig   string
	namespace    string
	chartPath    string
	releaseName  string
	ctx          context.Context
	testTimeout  = 10 * time.Minute
	pollInterval = 5 * time.Second
)

func TestHelm(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PostgreSQL Upgrade Helm Chart Suite")
}

var _ = BeforeSuite(func() {
	ctx = context.Background()

	// Create kind cluster for testing
	if os.Getenv("USE_EXISTING_CLUSTER") != "true" {
		By("Creating kind cluster for testing")
		err := createKindCluster("postgres-test")
		Expect(err).NotTo(HaveOccurred())

		By("Waiting for cluster to be ready")
		err = waitForClusterReady()
		Expect(err).NotTo(HaveOccurred())

		By("Loading required Docker images")
		err = loadDockerImages()
		Expect(err).NotTo(HaveOccurred())
	}

	// Get environment variables or use defaults
	kubeconfig = os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home := os.Getenv("HOME")
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	namespace = os.Getenv("TEST_NAMESPACE")
	if namespace == "" {
		namespace = fmt.Sprintf("postgres-test-%d", time.Now().Unix())
	}

	chartPath = os.Getenv("CHART_PATH")
	if chartPath == "" {
		chartPath = "../chart"
	}

	releaseName = "postgres-test"

	// Create namespace
	err := createNamespace(namespace)
	Expect(err).NotTo(HaveOccurred())

	// Ensure we have a clean state
	_ = helmDelete(releaseName, namespace)
})

var _ = AfterSuite(func() {
	if os.Getenv("SKIP_CLEANUP") != "true" {
		// Clean up Helm release and namespace
		_ = helmDelete(releaseName, namespace)
		_ = deleteNamespace(namespace)

		// Delete kind cluster if we created it
		if os.Getenv("USE_EXISTING_CLUSTER") != "true" {
			By("Deleting kind cluster")
			_ = deleteKindCluster("postgres-test")
		}
	}
})
