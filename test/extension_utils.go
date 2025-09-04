package test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// ExtensionTestConfig holds configuration for extension testing
type ExtensionTestConfig struct {
	// Extension mappings: config name -> actual extension name
	Extensions map[string]string
	// Database connection settings
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

// DefaultExtensionConfig returns the default extension configuration
func DefaultExtensionConfig() ExtensionTestConfig {
	return ExtensionTestConfig{
		Extensions: map[string]string{
			"pgvector":       "vector",
			"pgsodium":       "pgsodium",
			"pgjwt":          "pgjwt",
			"pgaudit":        "pgaudit",
			"pg_tle":         "pg_tle",
			"pg_stat_monitor": "pg_stat_monitor",
			"pg_repack":      "pg_repack",
			"pg_plan_filter": "pg_plan_filter",
			"pg_net":         "pg_net",
			"pg_jsonschema":  "jsonschema",
			"pg_hashids":     "hashids",
			"pg_cron":        "pg_cron",
			"pg-safeupdate":  "safeupdate",
			"index_advisor":  "index_advisor",
			"wal2json":       "wal2json",
			"hypopg":         "hypopg",
		},
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "postgres",
		Database: "postgres",
	}
}

// ExtensionTester provides comprehensive extension testing utilities
type ExtensionTester struct {
	config ExtensionTestConfig
	db     *sql.DB
}

// NewExtensionTester creates a new extension tester
func NewExtensionTester(config ExtensionTestConfig) (*ExtensionTester, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.Host, config.Port, config.User, config.Password, config.Database)
	
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	
	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	
	return &ExtensionTester{
		config: config,
		db:     db,
	}, nil
}

// Close closes the database connection
func (et *ExtensionTester) Close() error {
	if et.db != nil {
		return et.db.Close()
	}
	return nil
}

// VerifyAllExtensions checks that all expected extensions are installed
func (et *ExtensionTester) VerifyAllExtensions() (map[string]bool, error) {
	results := make(map[string]bool)
	
	for configName, extName := range et.config.Extensions {
		exists, err := et.IsExtensionInstalled(extName)
		if err != nil {
			return results, fmt.Errorf("failed to check extension %s (%s): %w", configName, extName, err)
		}
		results[configName] = exists
	}
	
	return results, nil
}

// IsExtensionInstalled checks if a specific extension is installed
func (et *ExtensionTester) IsExtensionInstalled(extensionName string) (bool, error) {
	var exists bool
	err := et.db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = $1)", extensionName).Scan(&exists)
	return exists, err
}

// GetInstalledExtensions returns all currently installed extensions
func (et *ExtensionTester) GetInstalledExtensions() (map[string]string, error) {
	extensions := make(map[string]string)
	
	rows, err := et.db.Query("SELECT extname, extversion FROM pg_extension ORDER BY extname")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	for rows.Next() {
		var name, version string
		if err := rows.Scan(&name, &version); err != nil {
			return nil, err
		}
		extensions[name] = version
	}
	
	return extensions, nil
}

// TestPgVectorFunctionality tests pgvector extension functionality
func (et *ExtensionTester) TestPgVectorFunctionality() error {
	// Create test table
	_, err := et.db.Exec("CREATE TABLE IF NOT EXISTS test_vectors_util (id SERIAL PRIMARY KEY, embedding vector(3))")
	if err != nil {
		return fmt.Errorf("failed to create vector table: %w", err)
	}
	
	// Insert test data
	_, err = et.db.Exec("INSERT INTO test_vectors_util (embedding) VALUES ('[1,2,3]'), ('[4,5,6]') ON CONFLICT DO NOTHING")
	if err != nil {
		return fmt.Errorf("failed to insert vector data: %w", err)
	}
	
	// Test similarity search
	var distance float64
	err = et.db.QueryRow("SELECT embedding <-> '[1,2,3]' FROM test_vectors_util ORDER BY embedding <-> '[1,2,3]' LIMIT 1").Scan(&distance)
	if err != nil {
		return fmt.Errorf("failed to perform similarity search: %w", err)
	}
	
	if distance < 0 {
		return fmt.Errorf("invalid distance result: %f", distance)
	}
	
	return nil
}

// TestPgCronFunctionality tests pg_cron extension functionality
func (et *ExtensionTester) TestPgCronFunctionality() error {
	jobName := "test-job-util"
	
	// Schedule a job
	_, err := et.db.Exec("SELECT cron.schedule($1, '* * * * *', 'SELECT 1;')", jobName)
	if err != nil {
		return fmt.Errorf("failed to schedule cron job: %w", err)
	}
	
	// Verify job was scheduled
	var count int
	err = et.db.QueryRow("SELECT COUNT(*) FROM cron.job WHERE jobname = $1", jobName).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check scheduled job: %w", err)
	}
	
	if count != 1 {
		return fmt.Errorf("expected 1 job, got %d", count)
	}
	
	// Clean up
	_, err = et.db.Exec("SELECT cron.unschedule($1)", jobName)
	if err != nil {
		return fmt.Errorf("failed to unschedule cron job: %w", err)
	}
	
	return nil
}

// TestPgSodiumFunctionality tests pgsodium extension functionality
func (et *ExtensionTester) TestPgSodiumFunctionality() error {
	// Test encryption/decryption
	var encrypted string
	err := et.db.QueryRow("SELECT encode(pgsodium.crypto_secretbox('test data', decode('test-key-32-bytes-long-for-sodium!', 'escape')), 'base64')").Scan(&encrypted)
	if err != nil {
		return fmt.Errorf("failed to encrypt data: %w", err)
	}
	
	if encrypted == "" {
		return fmt.Errorf("encryption returned empty result")
	}
	
	return nil
}

// TestPgJWTFunctionality tests pgjwt extension functionality
func (et *ExtensionTester) TestPgJWTFunctionality() error {
	// Test JWT signing
	var token string
	err := et.db.QueryRow("SELECT extensions.sign('{\"sub\":\"1234567890\",\"name\":\"John Doe\",\"iat\":1516239022}', 'secret')").Scan(&token)
	if err != nil {
		return fmt.Errorf("failed to sign JWT: %w", err)
	}
	
	if token == "" {
		return fmt.Errorf("JWT signing returned empty result")
	}
	
	// Verify JWT has proper structure (header.payload.signature)
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return fmt.Errorf("invalid JWT format, expected 3 parts, got %d", len(parts))
	}
	
	return nil
}

// TestJsonSchemaFunctionality tests jsonschema extension functionality
func (et *ExtensionTester) TestJsonSchemaFunctionality() error {
	var isValid bool
	err := et.db.QueryRow("SELECT jsonschema.json_matches_schema('{\"type\": \"object\"}', '{\"test\": 123}')").Scan(&isValid)
	if err != nil {
		return fmt.Errorf("failed to validate JSON schema: %w", err)
	}
	
	if !isValid {
		return fmt.Errorf("JSON schema validation failed unexpectedly")
	}
	
	return nil
}

// TestHashidsFunctionality tests hashids extension functionality
func (et *ExtensionTester) TestHashidsFunctionality() error {
	var hashid string
	err := et.db.QueryRow("SELECT hashids.encode(123, 'salt', 8)").Scan(&hashid)
	if err != nil {
		return fmt.Errorf("failed to encode hashid: %w", err)
	}
	
	if hashid == "" {
		return fmt.Errorf("hashid encoding returned empty result")
	}
	
	return nil
}

// ServiceTester provides utilities for testing integrated services
type ServiceTester struct {
	config ServiceTestConfig
}

// ServiceTestConfig holds configuration for service testing
type ServiceTestConfig struct {
	PostgreSQLHost string
	PostgreSQLPort int
	PgBouncerHost  string
	PgBouncerPort  int
	PostgRESTHost  string
	PostgRESTPort  int
	User           string
	Password       string
	Database       string
}

// DefaultServiceConfig returns the default service configuration
func DefaultServiceConfig() ServiceTestConfig {
	return ServiceTestConfig{
		PostgreSQLHost: "localhost",
		PostgreSQLPort: 5432,
		PgBouncerHost:  "localhost",
		PgBouncerPort:  6432,
		PostgRESTHost:  "localhost",
		PostgRESTPort:  3000,
		User:           "postgres",
		Password:       "postgres",
		Database:       "postgres",
	}
}

// NewServiceTester creates a new service tester
func NewServiceTester(config ServiceTestConfig) *ServiceTester {
	return &ServiceTester{config: config}
}

// TestPgBouncerConnection tests connection through PgBouncer
func (st *ServiceTester) TestPgBouncerConnection() error {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		st.config.PgBouncerHost, st.config.PgBouncerPort, st.config.User, st.config.Password, st.config.Database)
	
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to connect to PgBouncer: %w", err)
	}
	defer db.Close()
	
	// Test connection
	var result int
	err = db.QueryRow("SELECT 1").Scan(&result)
	if err != nil {
		return fmt.Errorf("failed to execute query through PgBouncer: %w", err)
	}
	
	if result != 1 {
		return fmt.Errorf("unexpected query result: %d", result)
	}
	
	return nil
}

// TestPgBouncerPools tests PgBouncer pool status
func (st *ServiceTester) TestPgBouncerPools() (map[string]interface{}, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s dbname=pgbouncer sslmode=disable",
		st.config.PgBouncerHost, st.config.PgBouncerPort, st.config.User)
	
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PgBouncer admin: %w", err)
	}
	defer db.Close()
	
	pools := make(map[string]interface{})
	rows, err := db.Query("SHOW POOLS")
	if err != nil {
		return nil, fmt.Errorf("failed to show pools: %w", err)
	}
	defer rows.Close()
	
	for rows.Next() {
		var database, user, clActive, clWaiting, svActive, svIdle, svUsed, svTested, svLogin, maxwait, maxwaitUs, poolMode string
		err := rows.Scan(&database, &user, &clActive, &clWaiting, &svActive, &svIdle, &svUsed, &svTested, &svLogin, &maxwait, &maxwaitUs, &poolMode)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pool data: %w", err)
		}
		
		pools[database] = map[string]string{
			"user":       user,
			"cl_active":  clActive,
			"cl_waiting": clWaiting,
			"sv_active":  svActive,
			"sv_idle":    svIdle,
			"pool_mode":  poolMode,
		}
	}
	
	return pools, nil
}

// TestPostgRESTAPI tests PostgREST API functionality
func (st *ServiceTester) TestPostgRESTAPI() error {
	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("http://%s:%d/", st.config.PostgRESTHost, st.config.PostgRESTPort)
	
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to connect to PostgREST: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("PostgREST returned status %d", resp.StatusCode)
	}
	
	return nil
}

// TestPostgRESTHealthCheck tests PostgREST health endpoint
func (st *ServiceTester) TestPostgRESTHealthCheck() (map[string]interface{}, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("http://%s:%d/rpc/health_check", st.config.PostgRESTHost, st.config.PostgRESTPort)
	
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to call health check: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("health check returned status %d", resp.StatusCode)
	}
	
	// For now, return basic info - in real implementation you'd parse JSON response
	return map[string]interface{}{
		"status": "ok",
		"code":   resp.StatusCode,
	}, nil
}

// LoadTester provides utilities for load testing extensions and services
type LoadTester struct {
	extensionTester *ExtensionTester
	serviceTester   *ServiceTester
}

// NewLoadTester creates a new load tester
func NewLoadTester(extConfig ExtensionTestConfig, svcConfig ServiceTestConfig) (*LoadTester, error) {
	extTester, err := NewExtensionTester(extConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create extension tester: %w", err)
	}
	
	svcTester := NewServiceTester(svcConfig)
	
	return &LoadTester{
		extensionTester: extTester,
		serviceTester:   svcTester,
	}, nil
}

// Close closes the load tester
func (lt *LoadTester) Close() error {
	if lt.extensionTester != nil {
		return lt.extensionTester.Close()
	}
	return nil
}

// RunConcurrentVectorSearches runs concurrent vector similarity searches
func (lt *LoadTester) RunConcurrentVectorSearches(concurrency, searchesPerWorker int) error {
	// Create test data if not exists
	_, err := lt.extensionTester.db.Exec(`
		CREATE TABLE IF NOT EXISTS load_test_vectors_util (
			id SERIAL PRIMARY KEY, 
			embedding vector(128), 
			category TEXT
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create vector table: %w", err)
	}
	
	// Insert sample data
	_, err = lt.extensionTester.db.Exec(`
		INSERT INTO load_test_vectors_util (embedding, category)
		SELECT 
			ARRAY(SELECT random()::float4 FROM generate_series(1, 128))::vector(128),
			CASE WHEN random() < 0.5 THEN 'category_a' ELSE 'category_b' END
		FROM generate_series(1, 100)
		ON CONFLICT DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("failed to insert vector data: %w", err)
	}
	
	// Run concurrent searches
	done := make(chan error, concurrency)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	for i := 0; i < concurrency; i++ {
		go func() {
			for j := 0; j < searchesPerWorker; j++ {
				var distance float64
				err := lt.extensionTester.db.QueryRowContext(ctx, `
					WITH query_vector AS (
						SELECT ARRAY(SELECT random()::float4 FROM generate_series(1, 128))::vector(128) as vec
					)
					SELECT embedding <-> query_vector.vec as distance
					FROM load_test_vectors_util, query_vector
					ORDER BY embedding <-> query_vector.vec
					LIMIT 1
				`).Scan(&distance)
				
				if err != nil {
					done <- fmt.Errorf("vector search failed: %w", err)
					return
				}
			}
			done <- nil
		}()
	}
	
	// Wait for all workers to complete
	for i := 0; i < concurrency; i++ {
		if err := <-done; err != nil {
			return err
		}
	}
	
	return nil
}

// RunConcurrentPgBouncerConnections tests PgBouncer under concurrent load
func (lt *LoadTester) RunConcurrentPgBouncerConnections(concurrency, queriesPerWorker int) error {
	done := make(chan error, concurrency)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	for i := 0; i < concurrency; i++ {
		go func(workerID int) {
			connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
				lt.serviceTester.config.PgBouncerHost, lt.serviceTester.config.PgBouncerPort,
				lt.serviceTester.config.User, lt.serviceTester.config.Password, lt.serviceTester.config.Database)
			
			db, err := sql.Open("postgres", connStr)
			if err != nil {
				done <- fmt.Errorf("worker %d: failed to connect: %w", workerID, err)
				return
			}
			defer db.Close()
			
			for j := 0; j < queriesPerWorker; j++ {
				var result int
				err := db.QueryRowContext(ctx, "SELECT $1", workerID*queriesPerWorker+j).Scan(&result)
				if err != nil {
					done <- fmt.Errorf("worker %d: query %d failed: %w", workerID, j, err)
					return
				}
			}
			done <- nil
		}(i)
	}
	
	// Wait for all workers to complete
	for i := 0; i < concurrency; i++ {
		if err := <-done; err != nil {
			return err
		}
	}
	
	return nil
}