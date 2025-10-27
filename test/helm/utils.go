package helm

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/flanksource/commons/test"
)

// createKindCluster creates a kind cluster
func createKindCluster(name string) error {
	fmt.Printf("Creating Kind Cluster: %s\n", name)

	// Check if cluster already exists
	cmd := exec.Command("kind", "get", "clusters")
	output, _ := cmd.Output()
	if strings.Contains(string(output), name) {
		fmt.Printf("Kind cluster %s already exists, reusing it\n", name)
		return nil
	}

	// Create kind config
	kindConfig := `
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: ` + name + `
nodes:
  - role: control-plane
    kubeadmConfigPatches:
      - |
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "node-type=control-plane"
    extraPortMappings:
      - containerPort: 30432
        hostPort: 5432
        protocol: TCP
`

	// Create cluster with config
	cmd = exec.Command("kind", "create", "cluster", "--name", name, "--config", "-")
	cmd.Stdin = strings.NewReader(kindConfig)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to create kind cluster: %w\nStdout: %s\nStderr: %s",
			err, stdout.String(), stderr.String())
	}

	fmt.Printf("Kind cluster %s created successfully\n", name)
	return nil
}

// deleteKindCluster deletes a kind cluster
func deleteKindCluster(name string) error {
	cmd := exec.Command("kind", "delete", "cluster", "--name", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Ignore error if cluster doesn't exist
		if strings.Contains(string(output), "not found") {
			return nil
		}
		return fmt.Errorf("failed to delete kind cluster: %w, output: %s", err, output)
	}
	return nil
}

// waitForClusterReady waits for the cluster to be ready
func waitForClusterReady() error {
	// Wait for nodes to be ready
	cmd := exec.Command("kubectl", "wait", "--for=condition=Ready",
		"nodes", "--all", "--timeout=120s")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed waiting for nodes: %w", err)
	}

	// Wait for system pods to be ready
	cmd = exec.Command("kubectl", "wait", "--for=condition=Ready",
		"pods", "--all", "-n", "kube-system", "--timeout=60s")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed waiting for system pods: %w", err)
	}

	return nil
}

// loadDockerImages loads required Docker images into kind
func loadDockerImages() error {
	fmt.Printf("Loading Docker Images into Kind\n")

	images := []string{
		"flanksource/postgres:latest",
		"postgres:14-bookworm",
		"postgres:15-bookworm",
		"postgres:16-bookworm",
		"postgres:17-bookworm",
	}

	for i, image := range images {
		fmt.Printf("[%d/%d] Processing image: %s\n", i+1, len(images), image)

		// First try to pull the image
		fmt.Printf("Attempting to pull image...\n")
		cmd := exec.Command("docker", "pull", image)
		if err := cmd.Run(); err != nil {
			fmt.Printf("Warning: Could not pull image %s (may use local)\n", image)
		}

		// Load into kind
		fmt.Printf("Loading image into kind cluster...\n")
		cmd = exec.Command("kind", "load", "docker-image", image, "--name", "postgres-test")
		if err := cmd.Run(); err != nil {
			fmt.Printf("Warning: Could not load image %s into kind\n", image)
		} else {
			fmt.Printf("✓ Image loaded successfully\n")
		}
	}

	fmt.Printf("Docker Images Loading Complete\n")
	return nil
}

// createNamespace creates a Kubernetes namespace
func createNamespace(name string) error {
	cmd := exec.Command("kubectl", "create", "namespace", name)
	output, err := cmd.CombinedOutput()
	if err != nil && strings.Contains(string(output), "already exists") {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to create namespace: %w, output: %s", err, output)
	}
	return nil
}

// deleteNamespace deletes a Kubernetes namespace
func deleteNamespace(name string) error {
	cmd := exec.Command("kubectl", "delete", "namespace", name, "--wait=false")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete namespace: %w", err)
	}
	return nil
}

// helmDelete deletes a Helm release using the fluent interface
func helmDelete(releaseName, namespace string) error {
	chart := test.NewHelmChart("").
		Release(releaseName).
		Namespace(namespace).
		Delete()

	return chart.Error()
}
