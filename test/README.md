# PostgreSQL Comprehensive Test Suite

This directory contains comprehensive tests for PostgreSQL with extensions, services integration, and upgrade capabilities.

## Overview

The test suite validates all aspects of PostgreSQL functionality including:

### Core Features
- PostgreSQL installations and upgrades (14→15→16→17)
- Admin password reset and management
- Dynamic configuration generation based on resources
- Production-grade settings and health monitoring

### Extensions (16 total)
- **pgvector** - Vector similarity search and embeddings
- **pgsodium** - Encryption and cryptographic functions
- **pgjwt** - JSON Web Token generation and validation
- **pgaudit** - Session and object audit logging
- **pg_tle** - Trusted Language Extensions
- **pg_stat_monitor** - Advanced query performance monitoring
- **pg_repack** - Online table reorganization
- **pg_plan_filter** - Query plan filtering
- **pg_net** - HTTP and network requests from SQL
- **pg_jsonschema** - JSON schema validation
- **pg_hashids** - Short unique ID generation
- **pg_cron** - Job scheduling within PostgreSQL
- **pg-safeupdate** - Safe UPDATE and DELETE operations
- **index_advisor** - Index recommendation engine
- **wal2json** - JSON output plugin for logical replication
- **hypopg** - Hypothetical indexes for query optimization

### Integrated Services
- **PgBouncer** - Connection pooling and management
- **PostgREST** - Automatic REST API generation
- **WAL-G** - Backup and recovery management
- **s6-overlay** - Process supervision and service management

### Testing Approaches
- **Docker Compose** - Local integration testing without Kubernetes
- **Kubernetes/Helm** - Full Helm chart testing with Ginkgo
- **Load Testing** - Performance testing under concurrent load
- **Extension Testing** - Individual extension functionality validation
- **Service Testing** - Integration testing of all services

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

### Task Runner Commands (Recommended)

The project uses Task for streamlined testing. All Task commands are defined in `Taskfile.test.yaml`.

#### View available tasks
```bash
task test:help
```

#### Run all tests (upgrades + features)
```bash
task test:all
```

#### Docker Compose Testing (No Kubernetes Required)

Start test environment:
```bash
task test:test-compose-up
```

Run integration tests:
```bash
task test:test-integration-docker
```

Run load tests:
```bash
task test:test-load
```

Stop test environment:
```bash
task test:test-compose-down
```

#### Extension Testing

Test all extensions:
```bash
task test:test-extensions
```

Verify extension installation:
```bash
task test:verify-extensions
```

#### Service Testing

Test all services (PgBouncer, PostgREST, WAL-G):
```bash
task test:test-services
```

Verify service health:
```bash
task test:verify-services
```

#### Performance Testing

Run benchmarks:
```bash
task test:benchmark
```

Test concurrent operations:
```bash
task test:test-load
```

#### Manual Testing

Start environment for manual testing:
```bash
task test:test-manual
# Services available at:
# - PostgreSQL: localhost:15432
# - PgBouncer: localhost:16432
# - PostgREST: localhost:13000
```

Start with pgAdmin included:
```bash
task test:test-manual-full
# Additional service:
# - pgAdmin: localhost:18080 (admin@example.com/testpass)
```

#### Upgrade Testing

Test all upgrade paths:
```bash
task test:all-upgrades
```

Test specific upgrade:
```bash
task test:upgrade-14-to-17
```

### Traditional Make Commands

#### Run all tests
```bash
make test
```

#### Run tests with verbose output
```bash
make test-verbose
```

#### Run specific test suite
```bash
make test-focus FOCUS="New Installation"
```

#### Run tests in kind cluster (creates cluster automatically)
```bash
make test-kind
```

## Test Suites

### Core PostgreSQL Tests

#### 1. New Installation
Tests basic installation scenarios:
- Installation with default values
- Persistent volume claim creation
- Database connectivity
- Configuration generation

#### 2. Configuration Generation
Tests dynamic configuration:
- Memory-based calculations for PostgreSQL settings
- Custom configuration application
- Resource-based tuning

#### 3. PostgreSQL Version Upgrade
Tests upgrade scenarios:
- Upgrade from PostgreSQL 14/15/16 to 17
- Data persistence during upgrades
- Extension compatibility across versions
- Automatic upgrade on startup

#### 4. Admin Password Management
Tests password handling:
- Password reset functionality
- Password preservation when reset is disabled
- Authentication verification

### Extension Tests

#### 5. PostgreSQL Extensions
Tests all 16 extensions:
- Extension installation and availability
- Basic functionality of each extension
- Extension version compatibility
- Extension dependencies

#### 6. Vector Extensions (pgvector)
Tests vector similarity search:
- Vector table creation and indexing
- Similarity search operations
- Distance calculations
- Concurrent vector operations

#### 7. Crypto Extensions (pgsodium, pgjwt)
Tests cryptographic functions:
- Data encryption/decryption
- JWT token generation and validation
- Cryptographic key management

#### 8. JSON Extensions (pg_jsonschema, pg_hashids)
Tests JSON processing:
- JSON schema validation
- Unique ID generation
- JSON manipulation functions

#### 9. Monitoring Extensions (pgaudit, pg_stat_monitor)
Tests monitoring and auditing:
- Query performance monitoring
- Audit logging functionality
- Performance statistics collection

### Service Integration Tests

#### 10. PgBouncer Integration
Tests connection pooling:
- Connection pool configuration
- Pool statistics and monitoring
- Concurrent connection handling
- Pool mode testing (transaction, session, statement)

#### 11. PostgREST API
Tests automatic REST API:
- API endpoint generation
- Database table access via REST
- Authentication and authorization
- Custom function endpoints

#### 12. WAL-G Backup
Tests backup functionality:
- WAL-G binary availability
- Backup configuration
- Basic backup operations (without cloud storage)

#### 13. Service Dependencies
Tests service supervision:
- s6-overlay process management
- Service startup order
- Service health monitoring
- Service restart capabilities

### Performance and Load Tests

#### 14. Extension Load Testing
Tests extensions under load:
- Concurrent vector similarity searches
- Multiple extension operations
- Mixed workload scenarios
- Performance benchmarking

#### 15. Connection Pool Load Testing
Tests PgBouncer under load:
- High concurrent connections
- Connection churn testing
- Pool saturation scenarios
- Performance metrics collection

#### 16. Mixed Extension Workloads
Tests multiple extensions simultaneously:
- Vector + crypto + JSON operations
- Cross-extension transactions
- Resource usage monitoring
- Performance degradation testing

### High Availability and Production Tests

#### 17. High Availability and Production Settings
Tests production configurations:
- Production-grade settings
- Resource limits and requests
- PodDisruptionBudget
- WAL configuration

#### 18. Monitoring and Health Checks
Tests comprehensive health monitoring:
- Liveness probes for all services
- Readiness probes
- Startup probes
- Extension health checks
- Service dependency health

#### 19. Edge Cases and Error Handling
Tests error scenarios:
- Invalid configuration handling
- Storage constraints
- Recovery from failures
- Extension loading failures
- Service restart scenarios

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
├── postgres_integration_test.go    # Docker-based integration tests
├── enhanced_integration_test.go    # Kubernetes/Helm tests with all features
├── upgrade_test.go                 # PostgreSQL upgrade tests
├── helm_suite_test.go              # Test suite setup and teardown
├── helm_test.go                    # Main Helm test scenarios
├── helm_basic_test.go              # Basic Helm functionality
├── utils.go                        # Kubernetes helper functions
├── docker_utils.go                 # Docker testing utilities
├── extension_utils.go              # Extension testing utilities
├── health_utils.go                 # Health checking utilities
├── load-tests/                     # Load testing scripts
│   ├── pgbouncer-load-test.sh     # PgBouncer load testing
│   └── extension-load-test.sh     # Extension load testing
├── sql/                           # SQL test data and scripts
│   ├── create-test-user.sql       # Test user creation
│   └── seed-test-data.sql         # Test data seeding
├── docker-compose.test.yml        # Docker Compose test environment
├── Taskfile.test.yaml             # Task runner test commands
├── Makefile                       # Traditional build automation
├── kind-config.yaml               # Kind cluster configuration
├── go.mod                         # Go module dependencies
└── README.md                      # This documentation
```

## Writing New Tests

### Docker Compose Tests

For extension and service integration tests, use Docker Compose approach:

1. Add test cases to `postgres_integration_test.go`:
```go
func TestNewExtension(t *testing.T) {
    config := DefaultExtensionConfig()
    config.Host = "localhost"
    config.Port = 5432
    
    tester, err := NewExtensionTester(config)
    require.NoError(t, err)
    defer tester.Close()
    
    // Test extension functionality
    err = tester.TestExtensionFunctionality()
    assert.NoError(t, err)
}
```

2. Use extension testing utilities:
```go
// Create extension tester
tester, err := NewExtensionTester(config)

// Verify all extensions
results, err := tester.VerifyAllExtensions()

// Test specific functionality
err = tester.TestPgVectorFunctionality()
err = tester.TestPgCronFunctionality()
```

3. Use service testing utilities:
```go
// Create service tester
serviceTester := NewServiceTester(serviceConfig)

// Test PgBouncer
err = serviceTester.TestPgBouncerConnection()

// Test PostgREST API
err = serviceTester.TestPostgRESTAPI()
```

4. Use health checking utilities:
```go
// Create health checker
healthChecker := NewHealthChecker(healthConfig)

// Check all services
healthy, results := healthChecker.IsSystemHealthy()

// Wait for services to be ready
healthy, results = healthChecker.WaitForSystemHealthy(5 * time.Minute)
```

### Kubernetes/Helm Tests

For Helm chart tests, add test cases to `enhanced_integration_test.go`:

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

Use helper functions from `utils.go`:
```go
// Install Helm chart
err := helmInstall(HelmOptions{...})

// Execute command in pod
output, err := execInPod(namespace, labelSelector, container, command)

// Wait for pod to be ready
err = waitForPodReady(namespace, labelSelector, timeout)
```

## Continuous Integration

The test suite runs automatically in GitHub Actions with multiple workflows:

### Main Workflows

1. **PostgreSQL Feature Tests** (`.github/workflows/feature-tests.yml`)
   - Extension testing (basic, vector, crypto, JSON, audit)
   - Service testing (PgBouncer, PostgREST, WAL-G, s6-overlay)
   - Load testing (concurrent operations, connection pools)
   - Integration testing (Docker Compose based)
   - Upgrade testing with extensions

2. **Helm Chart Ginkgo Tests** (`.github/workflows/helm-ginkgo-tests.yml`)
   - Kubernetes-based testing with enhanced features
   - Extension integration in Kubernetes
   - Service dependency testing
   - Mixed workload testing

3. **Build and Test** (`.github/workflows/build-and-test.yml`)
   - Multi-architecture Docker builds
   - Upgrade testing across all PostgreSQL versions
   - Data persistence validation

4. **Test PostgreSQL Upgrade Images** (`.github/workflows/test.yml`)
   - Basic upgrade functionality
   - Feature verification integration

### Trigger Conditions
- Push to main branch
- Pull requests to main branch
- Manual workflow dispatch
- Changes to test files, Dockerfiles, or configurations

### Test Matrix
Each workflow runs tests in parallel across multiple dimensions:
- **Extension tests**: Different extension categories
- **Service tests**: Individual service validation
- **Load tests**: Various concurrent scenarios
- **Upgrade tests**: Multiple version paths with extensions
- **Platform tests**: AMD64 and ARM64 architectures

### Test Environment Variables
```yaml
GO_VERSION: '1.21'
POSTGRES_VERSION: '17'
IMAGE_NAME: 'postgres-enhanced-test'
```

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