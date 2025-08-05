# PostgreSQL Upgrade Helm Chart - Ginkgo Test Suite

This directory contains comprehensive Ginkgo-based tests for the PostgreSQL Upgrade Helm chart.

## Overview

The test suite validates all aspects of the Helm chart including:
- New installations
- PostgreSQL version upgrades (14→15→16→17)
- Admin password reset and management
- Dynamic configuration generation based on resources
- Production-grade settings
- Health checks and monitoring
- Edge cases and error handling

## Prerequisites

- Go 1.21+
- Kubernetes cluster (kind, minikube, or real cluster)
- Helm 3.x
- kubectl
- Ginkgo v2

## Installation

```bash
# Install dependencies
make deps

# Or manually
go mod download
go install github.com/onsi/ginkgo/v2/ginkgo@latest
```

## Running Tests

### Run all tests
```bash
make test
```

### Run tests with verbose output
```bash
make test-verbose
```

### Run specific test suite
```bash
make test-focus FOCUS="New Installation"
```

### Run tests in kind cluster (creates cluster automatically)
```bash
make test-kind
```

### Run tests in watch mode (for development)
```bash
make test-watch
```

## Test Suites

### 1. New Installation
Tests basic installation scenarios:
- Installation with default values
- Persistent volume claim creation
- Database connectivity
- Configuration generation

### 2. Configuration Generation
Tests dynamic configuration:
- Memory-based calculations for PostgreSQL settings
- Custom configuration application
- Resource-based tuning

### 3. PostgreSQL Version Upgrade
Tests upgrade scenarios:
- Upgrade from PostgreSQL 15 to 17
- Data persistence during upgrades
- Automatic upgrade on startup

### 4. Admin Password Management
Tests password handling:
- Password reset functionality
- Password preservation when reset is disabled
- Authentication verification

### 5. High Availability and Production Settings
Tests production configurations:
- Production-grade settings
- Resource limits and requests
- PodDisruptionBudget
- WAL configuration

### 6. Monitoring and Health Checks
Tests health monitoring:
- Liveness probes
- Readiness probes
- Startup probes

### 7. Edge Cases and Error Handling
Tests error scenarios:
- Invalid configuration handling
- Storage constraints
- Recovery from failures

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `TEST_NAMESPACE` | Kubernetes namespace for tests | `postgres-test-{timestamp}` |
| `CHART_PATH` | Path to Helm chart | `../chart` |
| `KUBECONFIG` | Path to kubeconfig file | `~/.kube/config` |
| `SKIP_CLEANUP` | Skip cleanup after tests | `false` |

## Local Development

### Create kind cluster for testing
```bash
make kind-create
```

### Install Helm chart manually for debugging
```bash
make helm-install
```

### Uninstall Helm chart
```bash
make helm-uninstall
```

### Delete kind cluster
```bash
make kind-delete
```

## Test Structure

```
test/
├── helm_suite_test.go    # Test suite setup and teardown
├── helm_test.go           # Main test scenarios
├── utils.go               # Helper functions and utilities
├── Makefile              # Build and test automation
├── kind-config.yaml      # Kind cluster configuration
└── go.mod                # Go module dependencies
```

## Writing New Tests

To add new test scenarios:

1. Add test cases to `helm_test.go`:
```go
Context("Your Test Context", func() {
    It("should do something", func() {
        By("First step")
        // Test implementation
        
        By("Second step")
        // More test implementation
        
        Expect(result).To(Equal(expected))
    })
})
```

2. Use helper functions from `utils.go`:
```go
// Install Helm chart
err := helmInstall(HelmOptions{...})

// Execute command in pod
output, err := execInPod(namespace, labelSelector, container, command)

// Wait for pod to be ready
err = waitForPodReady(namespace, labelSelector, timeout)
```

## Continuous Integration

The test suite runs automatically in GitHub Actions:
- On push to main branch
- On pull requests
- Manual workflow dispatch

Each test suite runs in parallel in its own job for faster feedback.

## Debugging Failed Tests

### View test reports
```bash
make report
```

### Check pod logs
```bash
kubectl logs -n <namespace> -l app.kubernetes.io/name=postgres-upgrade
```

### Describe pods
```bash
kubectl describe pods -n <namespace>
```

### Access pod shell
```bash
kubectl exec -it -n <namespace> <pod-name> -- bash
```

## Coverage

Generate test coverage report:
```bash
make test-coverage
```

## Troubleshooting

### Tests timing out
- Increase timeout in test code or Makefile
- Check if cluster has sufficient resources
- Verify Docker images are available

### Permission denied errors
- Ensure kubectl has proper permissions
- Check RBAC settings in cluster

### Chart installation failures
- Verify chart path is correct
- Check values syntax
- Review Helm debug output: `helm install --debug`

## Contributing

1. Write tests following Ginkgo best practices
2. Ensure all tests pass locally
3. Add appropriate documentation
4. Submit pull request with test results

## License

Same as the parent project.