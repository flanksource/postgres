package helm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	commonsLogger "github.com/flanksource/commons/logger"

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

var logger commonsLogger.Logger

func findParentDir(dir string) string {
	currentDir, _ := os.Getwd()

	for {

		if _, ok := os.Stat(filepath.Join(currentDir, dir)); ok == nil {
			return filepath.Join(currentDir, dir)
		}
		if _, ok := os.Stat(filepath.Join(currentDir, ".git")); ok == nil {
			// Reached the git root, stop searching
			return currentDir
		}
		currentDir = filepath.Dir(currentDir)
	}
	return ""

}

func TestHelm(t *testing.T) {
	logger = commonsLogger.NewWithWriter(GinkgoWriter)
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

	chartPath = findParentDir("chart")

	releaseName = "postgres-test"

	logger.Infof("KUBECONFIG=%s ns=%s, chart=%s", kubeconfig, namespace, chartPath)

	if stat, err := os.Stat(kubeconfig); err != nil || stat.IsDir() {
		path, _ := filepath.Abs(kubeconfig)
		Skip(fmt.Sprintf("KUBECONFIG %s is not valid, skipping helm tests", path))
	}

	By("Creating test namespace")

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
