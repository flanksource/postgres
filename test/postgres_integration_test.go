package test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// PostgresIntegrationConfig holds configuration for comprehensive PostgreSQL testing
type PostgresIntegrationConfig struct {
	ImageName        string
	ContainerName    string
	PostgresPort     string
	PgBouncerPort    string
	PostgRESTPort    string
	Extensions       []string
	TestDatabase     string
	TestUser         string
	TestPassword     string
}

// PostgresIntegrationTest manages comprehensive PostgreSQL testing
type PostgresIntegrationTest struct {
	config       *PostgresIntegrationConfig
	client       *DockerClient
	healthClient *HealthClient
}

// NewPostgresIntegrationTest creates a new comprehensive PostgreSQL test instance
func NewPostgresIntegrationTest(config *PostgresIntegrationConfig) *PostgresIntegrationTest {
	return &PostgresIntegrationTest{
		config:       config,
		client:       NewDockerClient(true),
		healthClient: NewHealthClient("localhost", 8081), // pgconfig health endpoint
	}
}

// TestPostgresIntegration tests the complete PostgreSQL distribution
func TestPostgresIntegration(t *testing.T) {
	// Get random available ports
	ports, err := GetRandomPorts(3)
	if err != nil {
		t.Fatalf("Failed to allocate random ports: %v", err)
	}

	config := &PostgresIntegrationConfig{
		ImageName:     "postgres-upgrade:latest",
		ContainerName: fmt.Sprintf("postgres-integration-test-%d", time.Now().Unix()),
		PostgresPort:  fmt.Sprintf("%d", ports[0]),
		PgBouncerPort: fmt.Sprintf("%d", ports[1]),
		PostgRESTPort: fmt.Sprintf("%d", ports[2]),
		Extensions:    []string{"pgvector", "pgaudit", "pg_cron", "pgsodium", "pgjwt"},
		TestDatabase:  "postgres",
		TestUser:      "postgres",
		TestPassword:  "testpass",
	}

	integrationTest := NewPostgresIntegrationTest(config)

	// Skip build since it's already built by the task system

	// Start PostgreSQL with all services
	t.Run("StartPostgresWithServices", func(t *testing.T) {
		if err := integrationTest.startContainer(); err != nil {
			t.Fatalf("Failed to start PostgreSQL container: %v", err)
		}
	})

	// Test PostgreSQL basic functionality
	t.Run("TestBasicPostgreSQL", func(t *testing.T) {
		if err := integrationTest.testBasicPostgreSQL(); err != nil {
			t.Errorf("Basic PostgreSQL test failed: %v", err)
		}
	})

	// Test extensions
	t.Run("TestExtensions", func(t *testing.T) {
		if err := integrationTest.testExtensions(); err != nil {
			t.Errorf("Extensions test failed: %v", err)
		}
	})

	// Test PgBouncer
	t.Run("TestPgBouncer", func(t *testing.T) {
		if err := integrationTest.testPgBouncer(); err != nil {
			t.Errorf("PgBouncer test failed: %v", err)
		}
	})

	// Test PostgREST
	t.Run("TestPostgREST", func(t *testing.T) {
		if err := integrationTest.testPostgREST(); err != nil {
			t.Errorf("PostgREST test failed: %v", err)
		}
	})

	// Test s6-overlay service management
	t.Run("TestServiceManagement", func(t *testing.T) {
		if err := integrationTest.testServiceManagement(); err != nil {
			t.Errorf("Service management test failed: %v", err)
		}
	})

	// Test health checks
	t.Run("TestHealthChecks", func(t *testing.T) {
		if err := integrationTest.testHealthChecks(); err != nil {
			t.Errorf("Health checks test failed: %v", err)
		}
	})

	// Cleanup
	t.Cleanup(func() {
		integrationTest.cleanup()
	})
}

// buildImage builds the PostgreSQL Docker image
func (pit *PostgresIntegrationTest) buildImage() error {
	pit.client.runner.Printf(colorBlue, colorBold, "Building comprehensive PostgreSQL image...")

	result := pit.client.runner.RunCommand("docker", "build", "-t", pit.config.ImageName, ".")
	if result.ExitCode != 0 {
		return fmt.Errorf("failed to build image: %v", result.Err)
	}

	pit.client.runner.Infof("Successfully built PostgreSQL image")
	return nil
}

// startContainer starts PostgreSQL with all services enabled
func (pit *PostgresIntegrationTest) startContainer() error {
	pit.client.runner.Printf(colorBlue, colorBold, "Starting PostgreSQL with all services...")

	// Remove existing container if it exists
	pit.client.runner.RunCommandQuiet("docker", "rm", "-f", pit.config.ContainerName)

	// Start container with all services enabled
	args := []string{
		"run", "-d",
		"--name", pit.config.ContainerName,
		"-p", fmt.Sprintf("%s:5432", pit.config.PostgresPort),
		"-p", fmt.Sprintf("%s:6432", pit.config.PgBouncerPort),
		"-p", fmt.Sprintf("%s:3000", pit.config.PostgRESTPort),
		"-e", fmt.Sprintf("POSTGRES_DB=%s", pit.config.TestDatabase),
		"-e", fmt.Sprintf("POSTGRES_USER=%s", pit.config.TestUser),
		"-e", fmt.Sprintf("POSTGRES_PASSWORD=%s", pit.config.TestPassword),
		"-e", "RESET_PASSWORD=true",
		"-e", fmt.Sprintf("POSTGRES_EXTENSIONS=%s", strings.Join(pit.config.Extensions, ",")),
		"-e", "PGBOUNCER_ENABLED=true",
		"-e", "POSTGREST_ENABLED=true",
		"-e", "PGBOUNCER_POOL_MODE=transaction",
		"-e", "PGBOUNCER_MAX_CLIENT_CONN=100",
		"-e", "POSTGREST_DB_SCHEMAS=public",
		"-e", fmt.Sprintf("POSTGREST_DB_ANON_ROLE=%s", pit.config.TestUser),
		pit.config.ImageName,
	}

	result := pit.client.runner.RunCommand("docker", args...)
	if result.ExitCode != 0 {
		return fmt.Errorf("failed to start container: %v", result.Err)
	}

	// Wait for services to be ready
	pit.client.runner.Printf(colorGray, "", "Waiting for services to be ready...")
	if err := pit.waitForServices(); err != nil {
		return fmt.Errorf("services failed to start: %v", err)
	}

	pit.client.runner.Infof("✅ All services started successfully")
	return nil
}

// waitForServices waits for all services to be ready
func (pit *PostgresIntegrationTest) waitForServices() error {
	timeout := 120 * time.Second

	// First wait for ports to be available
	pit.client.runner.Printf(colorGray, "", "Waiting for service ports to be available...")
	
	pgPort, _ := strconv.Atoi(pit.config.PostgresPort)
	pgBouncerPort, _ := strconv.Atoi(pit.config.PgBouncerPort)
	postgrestPort, _ := strconv.Atoi(pit.config.PostgRESTPort)
	
	if err := WaitForPort("localhost", pgPort, timeout); err != nil {
		return fmt.Errorf("PostgreSQL port not available: %v", err)
	}
	
	if err := WaitForPort("localhost", pgBouncerPort, timeout); err != nil {
		return fmt.Errorf("PgBouncer port not available: %v", err)
	}
	
	if err := WaitForPort("localhost", postgrestPort, timeout); err != nil {
		return fmt.Errorf("PostgREST port not available: %v", err)
	}

	// Then wait for services to be ready
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		// Check PostgreSQL
		if pit.checkPostgresReady() {
			pit.client.runner.Printf(colorGray, "", "✅ PostgreSQL ready")
			
			// Check PgBouncer
			if pit.checkPgBouncerReady() {
				pit.client.runner.Printf(colorGray, "", "✅ PgBouncer ready")
				
				// Check PostgREST
				if pit.checkPostgRESTReady() {
					pit.client.runner.Printf(colorGray, "", "✅ PostgREST ready")
					return nil
				}
			}
		}
		
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("services did not become ready within timeout")
}

// checkPostgresReady checks if PostgreSQL is ready
func (pit *PostgresIntegrationTest) checkPostgresReady() bool {
	db, err := pit.connectToPostgres()
	if err != nil {
		return false
	}
	defer db.Close()

	var result int
	err = db.QueryRow("SELECT 1").Scan(&result)
	return err == nil && result == 1
}

// checkPgBouncerReady checks if PgBouncer is ready
func (pit *PostgresIntegrationTest) checkPgBouncerReady() bool {
	db, err := pit.connectToPgBouncer()
	if err != nil {
		return false
	}
	defer db.Close()

	var result int
	err = db.QueryRow("SELECT 1").Scan(&result)
	return err == nil && result == 1
}

// checkPostgRESTReady checks if PostgREST is ready
func (pit *PostgresIntegrationTest) checkPostgRESTReady() bool {
	resp, err := http.Get(fmt.Sprintf("http://localhost:%s/", pit.config.PostgRESTPort))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// testBasicPostgreSQL tests basic PostgreSQL functionality
func (pit *PostgresIntegrationTest) testBasicPostgreSQL() error {
	pit.client.runner.Printf(colorGray, "", "Testing basic PostgreSQL functionality...")

	db, err := pit.connectToPostgres()
	if err != nil {
		return fmt.Errorf("failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	// Test basic query
	var version string
	err = db.QueryRow("SELECT version()").Scan(&version)
	if err != nil {
		return fmt.Errorf("failed to get PostgreSQL version: %v", err)
	}

	if !strings.Contains(version, "PostgreSQL 17") {
		return fmt.Errorf("unexpected PostgreSQL version: %s", version)
	}

	// Create test table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS test_integration (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create test table: %v", err)
	}

	// Insert test data
	_, err = db.Exec("INSERT INTO test_integration (name) VALUES ('test1'), ('test2'), ('test3')")
	if err != nil {
		return fmt.Errorf("failed to insert test data: %v", err)
	}

	// Verify data
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM test_integration").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to count test data: %v", err)
	}

	if count != 3 {
		return fmt.Errorf("expected 3 records, got %d", count)
	}

	pit.client.runner.Printf(colorGray, "", "✅ Basic PostgreSQL functionality verified")
	return nil
}

// testExtensions tests PostgreSQL extensions using health endpoints
func (pit *PostgresIntegrationTest) testExtensions() error {
	pit.client.runner.Printf(colorGray, "", "Testing PostgreSQL extensions via health endpoints...")

	// Wait for health endpoint to be available
	if err := pit.healthClient.WaitForHealthEndpoint(30); err != nil {
		return fmt.Errorf("health endpoint not available: %v", err)
	}

	// Get extension status from health endpoint
	if err := pit.healthClient.ValidateExtensionsHealth(); err != nil {
		return fmt.Errorf("extension health validation failed: %v", err)
	}

	// Check specific extensions
	for _, ext := range pit.config.Extensions {
		installed, err := pit.healthClient.IsExtensionInstalled(ext)
		if err != nil {
			return fmt.Errorf("failed to check extension %s: %v", ext, err)
		}
		if !installed {
			return fmt.Errorf("extension %s is not installed", ext)
		}
		pit.client.runner.Printf(colorGray, "", "  ✅ Extension %s is installed", ext)
	}

	pit.client.runner.Printf(colorGray, "", "✅ All extensions tested successfully via health endpoints")
	return nil
}

// testSingleExtension tests a single PostgreSQL extension
func (pit *PostgresIntegrationTest) testSingleExtension(db *sql.DB, extension string) error {
	pit.client.runner.Printf(colorGray, "", "  Testing extension: %s", extension)

	// Check if extension is installed
	var exists bool
	extName := extension
	if extension == "pgvector" {
		extName = "vector" // Handle name mapping
	} else if extension == "pg-safeupdate" {
		extName = "safeupdate"
	}

	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = $1)", extName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check extension %s: %v", extension, err)
	}

	if !exists {
		return fmt.Errorf("extension %s is not installed", extension)
	}

	// Test extension-specific functionality
	switch extension {
	case "pgvector":
		return pit.testPgVector(db)
	case "pg_cron":
		return pit.testPgCron(db)
	case "pgaudit":
		// pgaudit doesn't have specific functions to test
		return nil
	case "pgsodium":
		return pit.testPgSodium(db)
	case "pgjwt":
		return pit.testPgJWT(db)
	default:
		// For other extensions, just verify they're installed
		return nil
	}
}

// testPgVector tests pgvector functionality
func (pit *PostgresIntegrationTest) testPgVector(db *sql.DB) error {
	// Create test table with vector column
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS test_vectors (id SERIAL PRIMARY KEY, embedding VECTOR(3))")
	if err != nil {
		return fmt.Errorf("failed to create vector table: %v", err)
	}

	// Insert test vectors
	_, err = db.Exec("INSERT INTO test_vectors (embedding) VALUES ('[1,2,3]'), ('[4,5,6]') ON CONFLICT DO NOTHING")
	if err != nil {
		return fmt.Errorf("failed to insert vectors: %v", err)
	}

	// Test vector similarity search
	var distance float64
	err = db.QueryRow("SELECT embedding <-> '[1,2,3]' FROM test_vectors ORDER BY embedding <-> '[1,2,3]' LIMIT 1").Scan(&distance)
	if err != nil {
		return fmt.Errorf("failed to perform vector search: %v", err)
	}

	if distance < 0 {
		return fmt.Errorf("invalid distance: %f", distance)
	}

	return nil
}

// testPgCron tests pg_cron functionality
func (pit *PostgresIntegrationTest) testPgCron(db *sql.DB) error {
	// Test basic pg_cron functionality
	var jobId int
	err := db.QueryRow("SELECT cron.schedule('test-job', '0 0 * * *', 'SELECT 1;')").Scan(&jobId)
	if err != nil {
		return fmt.Errorf("failed to schedule cron job: %v", err)
	}

	// Clean up the test job
	_, err = db.Exec("SELECT cron.unschedule($1)", jobId)
	if err != nil {
		return fmt.Errorf("failed to unschedule cron job: %v", err)
	}

	return nil
}

// testPgSodium tests pgsodium functionality
func (pit *PostgresIntegrationTest) testPgSodium(db *sql.DB) error {
	// Test basic encryption
	var encrypted []byte
	err := db.QueryRow("SELECT pgsodium.crypto_secretbox('Hello, World!', 'my-secret-key-32-bytes-length!')").Scan(&encrypted)
	if err != nil {
		return fmt.Errorf("failed to encrypt with pgsodium: %v", err)
	}

	if len(encrypted) == 0 {
		return fmt.Errorf("encryption returned empty result")
	}

	return nil
}

// testPgJWT tests pgjwt functionality
func (pit *PostgresIntegrationTest) testPgJWT(db *sql.DB) error {
	// Test JWT signing
	var token string
	err := db.QueryRow("SELECT sign('{}', 'secret')").Scan(&token)
	if err != nil {
		return fmt.Errorf("failed to sign JWT: %v", err)
	}

	if token == "" {
		return fmt.Errorf("JWT signing returned empty token")
	}

	return nil
}

// testPgBouncer tests PgBouncer via health endpoints
func (pit *PostgresIntegrationTest) testPgBouncer() error {
	pit.client.runner.Printf(colorGray, "", "Testing PgBouncer via health endpoints...")

	// Wait for PgBouncer to be ready
	if err := pit.healthClient.WaitForServiceRunning("pgbouncer", 20); err != nil {
		return fmt.Errorf("PgBouncer not ready: %v", err)
	}

	// Get service details
	pgbouncerDetails, err := pit.healthClient.GetServiceDetails("pgbouncer")
	if err != nil {
		return fmt.Errorf("failed to get PgBouncer details: %v", err)
	}

	// Validate service status
	if pgbouncerDetails.Status != "running" {
		return fmt.Errorf("PgBouncer status is %s, expected running", pgbouncerDetails.Status)
	}

	if !pgbouncerDetails.PortOpen {
		return fmt.Errorf("PgBouncer port %d is not accessible", pgbouncerDetails.Port)
	}

	pit.client.runner.Printf(colorGray, "", "  ✅ PgBouncer is running on port %d", pgbouncerDetails.Port)
	pit.client.runner.Printf(colorGray, "", "✅ PgBouncer functionality verified via health endpoints")
	return nil
}

// testPostgREST tests PostgREST via health endpoints
func (pit *PostgresIntegrationTest) testPostgREST() error {
	pit.client.runner.Printf(colorGray, "", "Testing PostgREST via health endpoints...")

	// Wait for PostgREST to be ready
	if err := pit.healthClient.WaitForServiceRunning("postgrest", 20); err != nil {
		return fmt.Errorf("PostgREST not ready: %v", err)
	}

	// Get service details
	postgrestDetails, err := pit.healthClient.GetServiceDetails("postgrest")
	if err != nil {
		return fmt.Errorf("failed to get PostgREST details: %v", err)
	}

	// Validate service status
	if postgrestDetails.Status != "running" {
		return fmt.Errorf("PostgREST status is %s, expected running", postgrestDetails.Status)
	}

	if !postgrestDetails.PortOpen {
		return fmt.Errorf("PostgREST port %d is not accessible", postgrestDetails.Port)
	}

	pit.client.runner.Printf(colorGray, "", "  ✅ PostgREST is running on port %d", postgrestDetails.Port)
	pit.client.runner.Printf(colorGray, "", "✅ PostgREST functionality verified via health endpoints")
	return nil
}

// testServiceManagement tests s6-overlay service management
func (pit *PostgresIntegrationTest) testServiceManagement() error {
	pit.client.runner.Printf(colorGray, "", "Testing service management...")

	// Check if s6-supervise processes are running
	result := pit.client.runner.RunCommand("docker", "exec", pit.config.ContainerName, "pgrep", "-f", "s6-supervise")
	if result.ExitCode != 0 {
		return fmt.Errorf("s6-supervise processes not found")
	}

	// Check PostgreSQL process
	result = pit.client.runner.RunCommand("docker", "exec", pit.config.ContainerName, "pgrep", "-f", "postgres")
	if result.ExitCode != 0 {
		return fmt.Errorf("PostgreSQL process not found")
	}

	// Check PgBouncer process
	result = pit.client.runner.RunCommand("docker", "exec", pit.config.ContainerName, "pgrep", "-f", "pgbouncer")
	if result.ExitCode != 0 {
		return fmt.Errorf("PgBouncer process not found")
	}

	// Check PostgREST process
	result = pit.client.runner.RunCommand("docker", "exec", pit.config.ContainerName, "pgrep", "-f", "postgrest")
	if result.ExitCode != 0 {
		return fmt.Errorf("PostgREST process not found")
	}

	pit.client.runner.Printf(colorGray, "", "✅ Service management verified")
	return nil
}

// testHealthChecks tests the health check scripts
func (pit *PostgresIntegrationTest) testHealthChecks() error {
	pit.client.runner.Printf(colorGray, "", "Testing health checks...")

	// Test extension health check
	result := pit.client.runner.RunCommand("docker", "exec", pit.config.ContainerName, "/scripts/extension-health.sh")
	if result.ExitCode != 0 {
		return fmt.Errorf("extension health check failed: %s", result.Stderr)
	}

	// Test service health check
	result = pit.client.runner.RunCommand("docker", "exec", pit.config.ContainerName, "/scripts/service-health.sh")
	if result.ExitCode != 0 {
		return fmt.Errorf("service health check failed: %s", result.Stderr)
	}

	pit.client.runner.Printf(colorGray, "", "✅ Health checks verified")
	return nil
}

// Helper methods for database connections

// connectToPostgres creates a connection to PostgreSQL
func (pit *PostgresIntegrationTest) connectToPostgres() (*sql.DB, error) {
	connStr := fmt.Sprintf("host=localhost port=%s user=%s password=%s dbname=%s sslmode=disable",
		pit.config.PostgresPort, pit.config.TestUser, pit.config.TestPassword, pit.config.TestDatabase)
	return sql.Open("postgres", connStr)
}

// connectToPgBouncer creates a connection through PgBouncer
func (pit *PostgresIntegrationTest) connectToPgBouncer() (*sql.DB, error) {
	connStr := fmt.Sprintf("host=localhost port=%s user=%s password=%s dbname=%s sslmode=disable",
		pit.config.PgBouncerPort, pit.config.TestUser, pit.config.TestPassword, pit.config.TestDatabase)
	return sql.Open("postgres", connStr)
}

// cleanup removes test containers and resources
func (pit *PostgresIntegrationTest) cleanup() {
	pit.client.runner.Printf(colorYellow, colorBold, "🧹 Cleaning up test containers...")

	// Stop and remove container
	pit.client.runner.RunCommandQuiet("docker", "rm", "-f", pit.config.ContainerName)

	pit.client.runner.Infof("✅ Cleanup completed")
}

// TestPostgresQuickIntegration runs a quick integration test
func TestPostgresQuickIntegration(t *testing.T) {
	// Get random available ports
	ports, err := GetRandomPorts(3)
	if err != nil {
		t.Fatalf("Failed to allocate random ports: %v", err)
	}

	config := &PostgresIntegrationConfig{
		ImageName:     "postgres-upgrade:latest",
		ContainerName: fmt.Sprintf("postgres-quick-test-%d", time.Now().Unix()),
		PostgresPort:  fmt.Sprintf("%d", ports[0]),
		PgBouncerPort: fmt.Sprintf("%d", ports[1]),
		PostgRESTPort: fmt.Sprintf("%d", ports[2]),
		Extensions:    []string{"pgvector", "pg_cron"},
		TestDatabase:  "postgres",
		TestUser:      "postgres",
		TestPassword:  "testpass",
	}

	integrationTest := NewPostgresIntegrationTest(config)

	// Quick tests
	t.Run("StartServices", func(t *testing.T) {
		if err := integrationTest.startContainer(); err != nil {
			t.Fatalf("Failed to start services: %v", err)
		}
	})

	t.Run("TestBasicFunctionality", func(t *testing.T) {
		if err := integrationTest.testBasicPostgreSQL(); err != nil {
			t.Errorf("Basic functionality test failed: %v", err)
		}
	})

	t.Run("TestQuickExtensions", func(t *testing.T) {
		db, err := integrationTest.connectToPostgres()
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer db.Close()

		// Quick extension tests
		for _, ext := range config.Extensions {
			var exists bool
			extName := ext
			if ext == "pgvector" {
				extName = "vector"
			}
			err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = $1)", extName).Scan(&exists)
			if err != nil {
				t.Errorf("Failed to check extension %s: %v", ext, err)
			}
			if !exists {
				t.Errorf("Extension %s not installed", ext)
			}
		}
	})

	// Cleanup
	t.Cleanup(func() {
		integrationTest.cleanup()
	})
}