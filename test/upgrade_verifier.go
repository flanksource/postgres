package test

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// UpgradeVerifierConfig holds configuration for upgrade verification
type UpgradeVerifierConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	Timeout  time.Duration
}

// DefaultUpgradeVerifierConfig returns default configuration
func DefaultUpgradeVerifierConfig() UpgradeVerifierConfig {
	return UpgradeVerifierConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "testpass",
		Database: "testdb",
		Timeout:  30 * time.Second,
	}
}

// UpgradeVerifier provides SQL-based verification for PostgreSQL upgrades
type UpgradeVerifier struct {
	config UpgradeVerifierConfig
	db     *sql.DB
}

// NewUpgradeVerifier creates a new upgrade verifier
func NewUpgradeVerifier(config UpgradeVerifierConfig) *UpgradeVerifier {
	return &UpgradeVerifier{
		config: config,
	}
}

// Connect establishes database connection with retry logic
func (uv *UpgradeVerifier) Connect() error {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		uv.config.Host, uv.config.Port, uv.config.User, uv.config.Password, uv.config.Database)

	maxRetries := 30
	retryDelay := time.Second

	for i := 0; i < maxRetries; i++ {
		db, err := sql.Open("postgres", connStr)
		if err != nil {
			time.Sleep(retryDelay)
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = db.PingContext(ctx)
		cancel()

		if err == nil {
			uv.db = db
			return nil
		}

		db.Close()
		time.Sleep(retryDelay)
	}

	return fmt.Errorf("failed to connect to database after %d retries", maxRetries)
}

// Close closes the database connection
func (uv *UpgradeVerifier) Close() error {
	if uv.db != nil {
		return uv.db.Close()
	}
	return nil
}

// VerifyVersion checks that PostgreSQL version matches expected version
func (uv *UpgradeVerifier) VerifyVersion(expectedVersion string) error {
	ctx, cancel := context.WithTimeout(context.Background(), uv.config.Timeout)
	defer cancel()

	var version string
	err := uv.db.QueryRowContext(ctx, "SHOW server_version").Scan(&version)
	if err != nil {
		return fmt.Errorf("failed to query server version: %w", err)
	}

	// Extract major version number from version string (e.g., "17.2" -> "17")
	parts := strings.Split(version, ".")
	if len(parts) == 0 {
		return fmt.Errorf("invalid version format: %s", version)
	}

	majorVersion := parts[0]
	if majorVersion != expectedVersion {
		return fmt.Errorf("version mismatch: expected %s, got %s", expectedVersion, majorVersion)
	}

	return nil
}

// VerifyTables checks that tables exist in the database
func (uv *UpgradeVerifier) VerifyTables(expectedMinCount int) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), uv.config.Timeout)
	defer cancel()

	var count int
	err := uv.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM pg_tables WHERE schemaname = 'public'").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to query table count: %w", err)
	}

	if count < expectedMinCount {
		return count, fmt.Errorf("expected at least %d tables, found %d", expectedMinCount, count)
	}

	return count, nil
}

// VerifyDataIntegrity checks that expected data exists in a table
func (uv *UpgradeVerifier) VerifyDataIntegrity(tableName string, expectedCount int) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), uv.config.Timeout)
	defer cancel()

	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)
	err := uv.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to query %s count: %w", tableName, err)
	}

	if count != expectedCount {
		return count, fmt.Errorf("data integrity check failed for %s: expected %d records, found %d",
			tableName, expectedCount, count)
	}

	return count, nil
}

// VerifyView checks that a view exists and returns expected data
func (uv *UpgradeVerifier) VerifyView(viewName, field string, expectedValue interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), uv.config.Timeout)
	defer cancel()

	query := fmt.Sprintf("SELECT %s FROM %s", field, viewName)
	var result interface{}
	err := uv.db.QueryRowContext(ctx, query).Scan(&result)
	if err != nil {
		return fmt.Errorf("failed to query view %s: %w", viewName, err)
	}

	// Convert result to string for comparison
	resultStr := fmt.Sprintf("%v", result)
	expectedStr := fmt.Sprintf("%v", expectedValue)

	if resultStr != expectedStr {
		return fmt.Errorf("view %s check failed: expected %s=%v, got %v",
			viewName, field, expectedValue, result)
	}

	return nil
}

// VerifyExtensions checks that expected extensions are installed
func (uv *UpgradeVerifier) VerifyExtensions(extensions []string) (map[string]bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), uv.config.Timeout)
	defer cancel()

	results := make(map[string]bool)

	for _, ext := range extensions {
		// Map extension config names to actual extension names
		extName := ext
		switch ext {
		case "pgvector":
			extName = "vector"
		case "pg-safeupdate":
			extName = "safeupdate"
		}

		var exists bool
		err := uv.db.QueryRowContext(ctx,
			"SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = $1)", extName).Scan(&exists)
		if err != nil {
			return results, fmt.Errorf("failed to check extension %s: %w", ext, err)
		}

		results[ext] = exists
		if !exists {
			return results, fmt.Errorf("extension %s (%s) is not installed", ext, extName)
		}
	}

	return results, nil
}

// VerifyPgVectorFunctionality tests pgvector extension functionality
func (uv *UpgradeVerifier) VerifyPgVectorFunctionality(tableName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), uv.config.Timeout)
	defer cancel()

	// Test vector distance calculation
	query := fmt.Sprintf("SELECT vector_data <-> '[1,1,1]' FROM %s LIMIT 1", tableName)
	var distance float64
	err := uv.db.QueryRowContext(ctx, query).Scan(&distance)
	if err != nil {
		return fmt.Errorf("pgvector functionality check failed: %w", err)
	}

	if distance < 0 {
		return fmt.Errorf("invalid vector distance: %f", distance)
	}

	return nil
}

// VerifyPgCronFunctionality tests pg_cron extension functionality
func (uv *UpgradeVerifier) VerifyPgCronFunctionality(expectedJobName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), uv.config.Timeout)
	defer cancel()

	var jobName string
	err := uv.db.QueryRowContext(ctx,
		"SELECT jobname FROM cron.job WHERE jobname = $1", expectedJobName).Scan(&jobName)
	if err != nil {
		return fmt.Errorf("pg_cron functionality check failed: %w", err)
	}

	if jobName != expectedJobName {
		return fmt.Errorf("expected cron job %s, got %s", expectedJobName, jobName)
	}

	return nil
}

// VerifyVectorIndex tests that vector index is working
func (uv *UpgradeVerifier) VerifyVectorIndex(tableName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), uv.config.Timeout)
	defer cancel()

	// Check that vector index search works
	query := fmt.Sprintf(
		"SELECT COUNT(*) FROM %s WHERE vector_data <-> '[1,2,3]' < 1", tableName)
	var count int
	err := uv.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return fmt.Errorf("vector index functionality check failed: %w", err)
	}

	// We just need to verify the query works, count can be any value >= 0
	if count < 0 {
		return fmt.Errorf("invalid count from vector index query: %d", count)
	}

	return nil
}

// VerifyBasicUpgrade performs all basic upgrade verification checks
func (uv *UpgradeVerifier) VerifyBasicUpgrade(expectedVersion, testDatabase string) error {
	// Check version
	if err := uv.VerifyVersion(expectedVersion); err != nil {
		return fmt.Errorf("version check failed: %w", err)
	}

	// Check tables exist (at least 1)
	tableCount, err := uv.VerifyTables(1)
	if err != nil {
		return fmt.Errorf("table check failed: %w", err)
	}
	fmt.Printf("✅ Tables are present (%d)\n", tableCount)

	// Check data integrity
	recordCount, err := uv.VerifyDataIntegrity("test_upgrade", 5)
	if err != nil {
		return fmt.Errorf("data integrity check failed: %w", err)
	}
	fmt.Printf("✅ Data integrity verified (%d records)\n", recordCount)

	// Check view
	if err := uv.VerifyView("test_upgrade_summary", "total_records", int64(5)); err != nil {
		return fmt.Errorf("view check failed: %w", err)
	}
	fmt.Printf("✅ View test_upgrade_summary is working\n")

	return nil
}

// VerifyUpgradeWithExtensions performs upgrade verification including extension checks
func (uv *UpgradeVerifier) VerifyUpgradeWithExtensions(
	expectedVersion, testDatabase string, extensions []string) error {

	// Check version
	if err := uv.VerifyVersion(expectedVersion); err != nil {
		return fmt.Errorf("version check failed: %w", err)
	}
	fmt.Printf("✅ PostgreSQL version is %s\n", expectedVersion)

	// Check extensions are installed
	results, err := uv.VerifyExtensions(extensions)
	if err != nil {
		return fmt.Errorf("extension check failed: %w", err)
	}
	for ext, installed := range results {
		if installed {
			fmt.Printf("✅ Extension %s is installed\n", ext)
		}
	}

	// Check pgvector functionality
	if err := uv.VerifyPgVectorFunctionality("test_upgrade_extensions"); err != nil {
		return fmt.Errorf("pgvector functionality check failed: %w", err)
	}
	fmt.Printf("✅ pgvector functionality verified\n")

	// Check pg_cron jobs
	if err := uv.VerifyPgCronFunctionality("cleanup-job"); err != nil {
		return fmt.Errorf("pg_cron functionality check failed: %w", err)
	}
	fmt.Printf("✅ pg_cron jobs preserved\n")

	// Check data integrity
	recordCount, err := uv.VerifyDataIntegrity("test_upgrade_extensions", 5)
	if err != nil {
		return fmt.Errorf("data integrity check failed: %w", err)
	}
	fmt.Printf("✅ Data integrity verified (%d records)\n", recordCount)

	// Check vector index
	if err := uv.VerifyVectorIndex("test_upgrade_extensions"); err != nil {
		return fmt.Errorf("vector index check failed: %w", err)
	}
	fmt.Printf("✅ Vector index functionality verified\n")

	// Check view
	if err := uv.VerifyView("test_upgrade_extensions_summary", "total_records", int64(5)); err != nil {
		return fmt.Errorf("view check failed: %w", err)
	}
	fmt.Printf("✅ View functionality verified\n")

	return nil
}

// WaitForPostgres waits for PostgreSQL to become available with retry logic
func WaitForPostgres(host string, port int, user, password, database string, maxWait time.Duration) error {
	config := UpgradeVerifierConfig{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		Database: database,
		Timeout:  maxWait,
	}

	verifier := NewUpgradeVerifier(config)

	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		if err := verifier.Connect(); err == nil {
			verifier.Close()
			return nil
		}
		time.Sleep(time.Second)
	}

	return fmt.Errorf("postgres did not become available within %v", maxWait)
}

// GetPostgresVersion returns the PostgreSQL version as an integer
func GetPostgresVersion(host string, port int, user, password, database string) (int, error) {
	config := UpgradeVerifierConfig{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		Database: database,
		Timeout:  10 * time.Second,
	}

	verifier := NewUpgradeVerifier(config)
	if err := verifier.Connect(); err != nil {
		return 0, err
	}
	defer verifier.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var version string
	err := verifier.db.QueryRowContext(ctx, "SHOW server_version").Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("failed to query server version: %w", err)
	}

	// Extract major version number
	parts := strings.Split(version, ".")
	if len(parts) == 0 {
		return 0, fmt.Errorf("invalid version format: %s", version)
	}

	majorVersion, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("failed to parse version %s: %w", parts[0], err)
	}

	return majorVersion, nil
}
