package test

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// PostgresUpgradeConfig holds configuration for PostgreSQL upgrade tests
type PostgresUpgradeConfig struct {
	ImageName      string
	Registry       string
	ImageBase      string
	SourceVersions []string
	TargetVersion  string
	TestUser       string
	TestPassword   string
	TestDatabase   string
}

// PostgresUpgradeTest manages PostgreSQL upgrade testing
type PostgresUpgradeTest struct {
	config *PostgresUpgradeConfig
	client *DockerClient
}

// NewPostgresUpgradeTest creates a new PostgreSQL upgrade test instance
func NewPostgresUpgradeTest(config *PostgresUpgradeConfig) *PostgresUpgradeTest {
	return &PostgresUpgradeTest{
		config: config,
		client: NewDockerClient(true),
	}
}

// TestPostgresUpgrade tests PostgreSQL upgrade functionality
func TestPostgresUpgrade(t *testing.T) {

	config := &PostgresUpgradeConfig{
		ImageName:      "postgres-upgrade:latest",
		Registry:       "ghcr.io",
		ImageBase:      "flanksource/postgres-upgrade",
		SourceVersions: []string{"14", "15", "16"},
		TargetVersion:  "17",
		TestUser:       "testuser",
		TestPassword:   "testpass",
		TestDatabase:   "testdb",
	}

	upgradeTest := NewPostgresUpgradeTest(config)

	// Build the upgrade image
	t.Run("BuildUpgradeImage", func(t *testing.T) {
		if err := upgradeTest.buildUpgradeImage(); err != nil {
			t.Fatalf("Failed to build upgrade image: %v", err)
		}
	})

	// Test upgrade paths
	upgradeMatrix := []struct {
		from string
		to   string
	}{
		{"14", "17"},
		{"15", "17"},
		{"16", "17"},
		{"15", "16"},
	}

	for _, upgrade := range upgradeMatrix {
		t.Run(fmt.Sprintf("Upgrade_%s_to_%s", upgrade.from, upgrade.to), func(t *testing.T) {
			if err := upgradeTest.testUpgrade(upgrade.from, upgrade.to); err != nil {
				t.Errorf("Upgrade from %s to %s failed: %v", upgrade.from, upgrade.to, err)
			}
		})
	}

	// Cleanup test volumes
	t.Cleanup(func() {
		upgradeTest.cleanup()
	})
}

// buildUpgradeImage builds the PostgreSQL upgrade Docker image
func (put *PostgresUpgradeTest) buildUpgradeImage() error {
	put.client.runner.Printf(colorBlue, colorBold, "Building PostgreSQL upgrade image...")

	// Check if Dockerfile exists in the current directory
	if _, err := os.Stat("Dockerfile"); os.IsNotExist(err) {
		// Try the modules directory
		dockerfilePath := "/Users/moshe/go/src/github.com/flanksource/docs-fix/modules/docker-postgres-upgrade"
		if _, err := os.Stat(dockerfilePath + "/Dockerfile"); err != nil {
			return fmt.Errorf("Dockerfile not found in current directory or modules path")
		}

		// Change to the docker-postgres-upgrade directory for building
		originalDir, _ := os.Getwd()
		if err := os.Chdir(dockerfilePath); err != nil {
			return fmt.Errorf("failed to change to dockerfile directory: %w", err)
		}
		defer os.Chdir(originalDir)
	}

	result := put.client.runner.RunCommand("docker", "build", "-t", put.config.ImageName, ".")
	if result.ExitCode != 0 {
		return fmt.Errorf("failed to build image: %v", result.Err)
	}

	put.client.runner.Infof("Successfully built upgrade image")
	return nil
}

// testUpgrade tests a specific upgrade path
func (put *PostgresUpgradeTest) testUpgrade(fromVersion, toVersion string) error {
	put.client.runner.Printf(colorBlue, colorBold, "Testing upgrade from PostgreSQL %s to %s", fromVersion, toVersion)

	// Create test volume for upgrade
	testVolumeName := fmt.Sprintf("pg%s-to-%s-test-%d", fromVersion, toVersion, time.Now().Unix())
	testVolume, err := put.createAndSeedVolume(fromVersion, testVolumeName)
	if err != nil {
		return fmt.Errorf("failed to create and seed test volume: %w", err)
	}

	// Run the upgrade
	if err := put.runUpgrade(testVolume, fromVersion, toVersion); err != nil {
		// Preserve volume on failure for debugging
		put.client.runner.Errorf("❌ Upgrade failed, preserving volume %s for debugging", testVolume.Name)
		put.client.runner.Errorf("To inspect: docker run --rm -it -v %s:/var/lib/postgresql/data postgres:%s bash", testVolume.Name, toVersion)
		return fmt.Errorf("upgrade failed: %w", err)
	}

	// Verify the upgrade
	if err := put.verifyUpgrade(testVolume, fromVersion, toVersion); err != nil {
		// Preserve volume on failure for debugging
		put.client.runner.Errorf("❌ Verification failed, preserving volume %s for debugging", testVolume.Name)
		put.client.runner.Errorf("To inspect: docker run --rm -it -v %s:/var/lib/postgresql/data postgres:%s bash", testVolume.Name, toVersion)
		return fmt.Errorf("upgrade verification failed: %w", err)
	}

	// Only delete volume on success
	put.client.runner.Infof("✅ Upgrade from %s to %s successful", fromVersion, toVersion)
	if err := testVolume.Delete(); err != nil {
		put.client.runner.Printf(colorGray, "", "Note: Failed to delete test volume %s: %v", testVolume.Name, err)
	}
	return nil
}

// createAndSeedVolume creates a volume and seeds it with test data
func (put *PostgresUpgradeTest) createAndSeedVolume(version string, volumeName ...string) (*Volume, error) {
	var name string
	if len(volumeName) > 0 {
		name = volumeName[0]
	} else {
		name = fmt.Sprintf("pg%s-test-data-%d", version, time.Now().Unix())
	}

	// Create new volume
	volume, err := CreateVolume(VolumeOptions{
		Name: name,
		Labels: map[string]string{
			"postgres.version": version,
			"test":             "true",
			"created":          time.Now().Format(time.RFC3339),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create volume: %w", err)
	}

	// Seed the volume using mounted script
	if err := put.seedVolumeWithScript(volume, version); err != nil {
		// Don't delete on failure - preserve for debugging
		put.client.runner.Errorf("Failed to seed volume %s, preserving for debugging", volume.Name)
		return nil, fmt.Errorf("failed to seed volume: %w", err)
	}

	return volume, nil
}

// seedVolumeWithScript seeds a PostgreSQL volume with test data using mounted scripts
func (put *PostgresUpgradeTest) seedVolumeWithScript(volume *Volume, version string) error {
	put.client.runner.Printf(colorGray, "", "🌱 Seeding PostgreSQL %s volume with mounted script...", version)

	// Create seed SQL script
	seedScript := put.createSeedScript()
	
	// Create temporary file for seed script
	tmpFile, err := os.CreateTemp("", "seed-*.sql")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	
	if _, err := tmpFile.WriteString(seedScript); err != nil {
		return fmt.Errorf("failed to write seed script: %w", err)
	}
	tmpFile.Close()

	containerName := fmt.Sprintf("postgres-seed-%s-%d", version, time.Now().Unix())

	// Start PostgreSQL container with seed script mounted
	// PostgreSQL automatically runs scripts in /docker-entrypoint-initdb.d/
	container, err := Run(ContainerOptions{
		Name:  containerName,
		Image: fmt.Sprintf("postgres:%s", version),
		Env: map[string]string{
			"POSTGRES_PASSWORD": put.config.TestPassword,
			"POSTGRES_DB":       put.config.TestDatabase,
			"POSTGRES_USER":     put.config.TestUser,
		},
		Volumes: map[string]string{
			volume.Name:    "/var/lib/postgresql/data",
			tmpFile.Name(): "/docker-entrypoint-initdb.d/01-seed.sql:ro",
		},
		Detach: true,
		Remove: false, // Don't auto-remove on failure for debugging
	})
	if err != nil {
		return fmt.Errorf("failed to start postgres container: %w", err)
	}

	// Wait for PostgreSQL to be ready and seed to complete
	put.client.runner.Statusf("⏳ Waiting for PostgreSQL %s to initialize and seed...", version)
	maxRetries := 60
	for i := 0; i < maxRetries; i++ {
		output, err := container.Exec("pg_isready", "-U", put.config.TestUser)
		if err == nil && strings.Contains(output, "accepting connections") {
			// Give it a bit more time for seed script to complete
			time.Sleep(3 * time.Second)
			put.client.runner.Successf("✅ PostgreSQL %s is ready and seeded", version)
			break
		}
		if i == maxRetries-1 {
			logs := container.Logs()
			put.client.runner.Errorf("Container logs:\n%s", logs)
			return fmt.Errorf("PostgreSQL %s failed to start and seed after %d seconds", version, maxRetries)
		}
		time.Sleep(1 * time.Second)
	}

	// Verify seed completed successfully
	logs := container.Logs()
	if strings.Contains(logs, "ERROR") && !strings.Contains(logs, "already exists") {
		put.client.runner.Errorf("Seed script errors found in logs:\n%s", logs)
		return fmt.Errorf("seed script failed")
	}

	// Clean up container on success
	container.Delete()
	put.client.runner.Printf(colorGray, "", "✅ Volume seeded successfully")
	return nil
}

// createSeedScript generates the SQL script for seeding test data
func (put *PostgresUpgradeTest) createSeedScript() string {
	return fmt.Sprintf(`-- Test seed data for PostgreSQL upgrade testing
CREATE TABLE IF NOT EXISTS test_upgrade (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100),
    value INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO test_upgrade (name, value) VALUES
    ('record1', 100),
    ('record2', 200),
    ('record3', 300),
    ('record4', 400),
    ('record5', 500);

-- Create some additional test objects
CREATE INDEX idx_test_upgrade_name ON test_upgrade(name);
CREATE INDEX idx_test_upgrade_value ON test_upgrade(value);

-- Create a view
CREATE VIEW test_upgrade_summary AS
SELECT COUNT(*) as total_records, SUM(value) as total_value
FROM test_upgrade;

-- Create a test user if not exists
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '%s') THEN
        CREATE USER %s WITH PASSWORD '%s';
    END IF;
END$$;

GRANT ALL PRIVILEGES ON DATABASE %s TO %s;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO %s;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO %s;

-- Add some PostgreSQL version-specific features if needed
DO $$
BEGIN
    RAISE NOTICE 'Seed script completed successfully';
END $$;
`, put.config.TestUser, put.config.TestUser, put.config.TestPassword,
   put.config.TestDatabase, put.config.TestUser,
   put.config.TestUser, put.config.TestUser)
}

// seedVolume seeds a PostgreSQL volume with test data
func (put *PostgresUpgradeTest) seedVolume(volume *Volume, version string) error {
	put.client.runner.Printf(colorGray, "", "🌱 Seeding PostgreSQL %s volume...", version)

	containerName := fmt.Sprintf("postgres-seed-%s-%d", version, time.Now().Unix())

	// Start PostgreSQL container
	container, err := Run(ContainerOptions{
		Name:  containerName,
		Image: fmt.Sprintf("postgres:%s", version),
		Env: map[string]string{
			"POSTGRES_PASSWORD": put.config.TestPassword,
			"POSTGRES_DB":       put.config.TestDatabase,
		},
		Volumes: map[string]string{
			volume.Name: "/var/lib/postgresql/data",
		},
		Detach: true,
		Remove: false, // Don't auto-remove for debugging
	})
	if err != nil {
		return fmt.Errorf("failed to start PostgreSQL container: %w", err)
	}

	// Wait for PostgreSQL to be ready
	put.client.runner.Statusf("⏳ Waiting for PostgreSQL %s to be ready...", version)
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		output, err := container.Exec("pg_isready", "-U", "postgres")
		if err == nil && strings.Contains(output, "accepting connections") {
			put.client.runner.Successf("✅ PostgreSQL %s is ready", version)
			break
		}
		if i == maxRetries-1 {
			return fmt.Errorf("PostgreSQL %s failed to start after %d seconds", version, maxRetries)
		}
		time.Sleep(1 * time.Second)
	}

	// Create test data
	put.client.runner.Infof("📊 Creating test data for PostgreSQL %s...", version)

	sqlCommands := fmt.Sprintf(`
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '%s') THEN
        CREATE USER %s WITH PASSWORD '%s';
    END IF;
END
$$;

GRANT ALL PRIVILEGES ON DATABASE %s TO %s;

DROP TABLE IF EXISTS test_table CASCADE;
CREATE TABLE test_table (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    version VARCHAR(10) DEFAULT '%s'
);

INSERT INTO test_table (name) VALUES
    ('Test Record 1 from PG%s'),
    ('Test Record 2 from PG%s'),
    ('Test Record 3 from PG%s');

DROP TABLE IF EXISTS version_info CASCADE;
CREATE TABLE version_info (
    id INTEGER PRIMARY KEY,
    original_version VARCHAR(10),
    seed_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO version_info (id, original_version) VALUES (1, '%s');
`, put.config.TestUser, put.config.TestUser, put.config.TestPassword,
		put.config.TestDatabase, put.config.TestUser,
		version, version, version, version, version)

	_, err = container.Exec("psql", "-U", "postgres", "-d", put.config.TestDatabase, "-c", sqlCommands)
	if err != nil {
		return fmt.Errorf("failed to create test data: %w", err)
	}

	// Verify the seed
	if err := put.verifySeedData(volume, version); err != nil {
		// Don't delete container on failure for debugging
		return fmt.Errorf("seed verification failed: %w", err)
	}

	// Clean up container on success
	container.Delete()
	put.client.runner.Successf("✅ Successfully seeded PostgreSQL %s volume", version)
	return nil
}

// verifySeedData verifies that a volume contains the expected seed data
func (put *PostgresUpgradeTest) verifySeedData(volume *Volume, version string) error {
	put.client.runner.Infof("🔍 Verifying seed data for PostgreSQL %s...", version)

	containerName := fmt.Sprintf("verify-seed-%s-%d", version, time.Now().Unix())

	// Use direct docker run command to avoid container management issues
	result := put.client.runner.RunCommand("docker", "run", 
		"--name", containerName,
		"-u", "postgres",
		"-v", fmt.Sprintf("%s:/var/lib/postgresql/data", volume.Name),
		fmt.Sprintf("postgres:%s", version),
		"bash", "-c", fmt.Sprintf(`
pg_ctl -D /var/lib/postgresql/data start -w &&
psql -U postgres -d %s -t -c 'SELECT COUNT(*) FROM test_table;' | tr -d ' ' &&
echo "SEPARATOR" &&
psql -U postgres -d %s -t -c 'SELECT original_version FROM version_info WHERE id = 1;' | tr -d ' ' &&
pg_ctl -D /var/lib/postgresql/data stop -w
`, put.config.TestDatabase, put.config.TestDatabase))
	
	// Clean up container regardless of result
	defer put.client.runner.RunCommandQuiet("docker", "rm", "-f", containerName)
	
	if result.ExitCode != 0 {
		return fmt.Errorf("verification container failed: %v", result.Err)
	}
	
	// The output should be in stdout/stderr from the run command
	logs := result.Stdout + result.Stderr

	// Parse output
	parts := strings.Split(logs, "SEPARATOR")
	if len(parts) < 2 {
		return fmt.Errorf("unexpected output format")
	}

	recordCount := strings.TrimSpace(parts[0])
	if !strings.Contains(recordCount, "3") {
		return fmt.Errorf("expected 3 records, found: %s", recordCount)
	}

	originalVersion := strings.TrimSpace(parts[1])
	if !strings.Contains(originalVersion, version) {
		return fmt.Errorf("expected version %s, found: %s", version, originalVersion)
	}

	put.client.runner.Infof("✅ Seed verification passed for PostgreSQL %s", version)
	return nil
}

// runUpgrade runs the PostgreSQL upgrade process
func (put *PostgresUpgradeTest) runUpgrade(volume *Volume, fromVersion, toVersion string) error {
	put.client.runner.Statusf("🚀 Starting upgrade from PostgreSQL %s to %s...", fromVersion, toVersion)

	// Validate versions
	fromInt := versionToInt(fromVersion)
	toInt := versionToInt(toVersion)
	if fromInt >= toInt {
		return fmt.Errorf("FROM_VERSION must be less than TO_VERSION")
	}

	// Build the appropriate image name
	imageName := put.config.ImageName
	if toVersion != "17" {
		imageName = fmt.Sprintf("%s/%s:to-%s", put.config.Registry, put.config.ImageBase, toVersion)
	}

	// Run the upgrade container  
	containerName := fmt.Sprintf("upgrade-%s-to-%s-%d", fromVersion, toVersion, time.Now().Unix())
	
	// Run upgrade with docker run to get proper exit code and output
	args := []string{
		"run",
		"--name", containerName,
		"--user", "postgres",
		"-e", fmt.Sprintf("PG_VERSION=%s", toVersion),
		"-e", "AUTO_UPGRADE=true",
		"-v", fmt.Sprintf("%s:/var/lib/postgresql/data", volume.Name),
		"-w", "/var/lib/postgresql",
		imageName,
	}
	
	put.client.runner.Printf(colorGray, "", "Running upgrade container...")
	result := put.client.runner.RunCommand("docker", args...)
	
	// Always try to get logs, even on failure
	logsResult := put.client.runner.RunCommandQuiet("docker", "logs", containerName)
	
	// Clean up container
	put.client.runner.RunCommandQuiet("docker", "rm", "-f", containerName)
	
	// Check result
	if result.ExitCode != 0 {
		put.client.runner.Errorf("Upgrade failed with exit code %d", result.ExitCode)
		put.client.runner.Errorf("STDOUT:\n%s", result.Stdout)
		put.client.runner.Errorf("STDERR:\n%s", result.Stderr)
		put.client.runner.Errorf("Container logs:\n%s", logsResult.Stdout)
		return fmt.Errorf("upgrade process failed with exit code %d", result.ExitCode)
	}
	
	// Check for error indicators in output
	allOutput := result.Stdout + result.Stderr + logsResult.Stdout
	if strings.Contains(allOutput, "ERROR") || strings.Contains(allOutput, "FATAL") {
		// Filter out expected non-error messages
		if !strings.Contains(allOutput, "upgrade completed successfully") {
			put.client.runner.Errorf("Upgrade output contains errors:\n%s", allOutput)
			return fmt.Errorf("upgrade process reported errors")
		}
	}

	put.client.runner.Infof("✅ Upgrade process completed")
	return nil
}

// verifyUpgrade verifies that the upgrade was successful
func (put *PostgresUpgradeTest) verifyUpgrade(volume *Volume, fromVersion, toVersion string) error {
	put.client.runner.Printf(colorGray, "", "🔍 Verifying upgrade from PostgreSQL %s to %s...", fromVersion, toVersion)

	containerName := fmt.Sprintf("verify-upgrade-%s-to-%s-%d", fromVersion, toVersion, time.Now().Unix())

	// Use direct docker run command to get output  
	result := put.client.runner.RunCommand("docker", "run",
		"--name", containerName,
		"-u", "postgres", 
		"-v", fmt.Sprintf("%s:/var/lib/postgresql/data", volume.Name),
		fmt.Sprintf("postgres:%s", toVersion),
		"bash", "-c", fmt.Sprintf(`
# Start PostgreSQL
pg_ctl -D /var/lib/postgresql/data start -w || exit 1

# Check version
actual_version=$(psql -U postgres -t -c "SHOW server_version;" | sed "s/^ *//" | grep -oE "^[0-9]+" | head -1)
if [ "$actual_version" != "%s" ]; then
    echo "ERROR: Version mismatch. Expected %s, got $actual_version"
    exit 1
fi
echo "✅ PostgreSQL version is %s"

# Check tables exist
table_count=$(psql -U postgres -d %s -t -c "SELECT COUNT(*) FROM pg_tables WHERE schemaname = 'public';" | tr -d " ")
if [ "$table_count" -lt 2 ]; then
    echo "ERROR: Tables missing. Expected at least 2, found $table_count"
    exit 1
fi
echo "✅ Tables are present"

# Check data integrity
record_count=$(psql -U postgres -d %s -t -c "SELECT COUNT(*) FROM test_table;" | tr -d " ")
if [ "$record_count" != "3" ]; then
    echo "ERROR: Data integrity check failed. Expected 3 records, found $record_count"
    exit 1
fi
echo "✅ Data integrity verified"

# Check original version info
original_version=$(psql -U postgres -d %s -t -c "SELECT original_version FROM version_info WHERE id = 1;" | tr -d " ")
if [ "$original_version" != "%s" ]; then
    echo "ERROR: Original version info corrupted. Expected %s, found $original_version"
    exit 1
fi
echo "✅ Original version info preserved"

# Display sample data
echo "Sample data after upgrade:"
psql -U postgres -d %s -c "SELECT * FROM test_table LIMIT 2;"

# Stop PostgreSQL
pg_ctl -D /var/lib/postgresql/data stop -w
`, toVersion, toVersion, toVersion, put.config.TestDatabase, put.config.TestDatabase,
			put.config.TestDatabase, fromVersion, fromVersion, put.config.TestDatabase))

	// Clean up container regardless of result
	defer put.client.runner.RunCommandQuiet("docker", "rm", "-f", containerName)

	if result.ExitCode != 0 {
		return fmt.Errorf("verification failed with exit code %d: %s", result.ExitCode, result.Stderr)
	}

	// Get the output from the command result
	logs := result.Stdout + result.Stderr

	// Check for errors
	if strings.Contains(logs, "ERROR:") {
		return fmt.Errorf("verification failed: %s", logs)
	}

	// Ensure all checks passed
	requiredChecks := []string{
		"PostgreSQL version is",
		"Tables are present",
		"Data integrity verified",
		"Original version info preserved",
	}

	for _, check := range requiredChecks {
		if !strings.Contains(logs, check) {
			return fmt.Errorf("verification check '%s' not found in logs", check)
		}
	}

	put.client.runner.Infof("✅ Upgrade verification passed!")
	return nil
}

// cleanup removes all test volumes
func (put *PostgresUpgradeTest) cleanup() {
	put.client.runner.Printf(colorYellow, colorBold, "🧹 Cleaning up test volumes...")

	// List all volumes and remove test volumes
	volumes, err := ListVolumes()
	if err != nil {
		put.client.runner.Printf(colorRed, "", "Failed to list volumes: %v", err)
		return
	}

	for _, volume := range volumes {
		// Remove PostgreSQL test data volumes
		if strings.HasPrefix(volume.Name, "pg") && strings.Contains(volume.Name, "-test-") {
			put.client.runner.Printf(colorGray, "", "Removing volume: %s", volume.Name)
			volume.Delete()
		}
		// Remove base test volumes
		for _, version := range put.config.SourceVersions {
			if volume.Name == fmt.Sprintf("pg%s-test-data", version) {
				put.client.runner.Printf(colorGray, "", "Removing volume: %s", volume.Name)
				volume.Delete()
			}
		}
	}

	put.client.runner.Infof("✅ Cleanup completed")
}

// versionToInt converts a PostgreSQL version string to an integer for comparison
func versionToInt(version string) int {
	switch version {
	case "14":
		return 14
	case "15":
		return 15
	case "16":
		return 16
	case "17":
		return 17
	default:
		return 0
	}
}

// TestPostgresUpgradeQuick runs a quick test (14 to 17 only)
func TestPostgresUpgradeQuick(t *testing.T) {

	config := &PostgresUpgradeConfig{
		ImageName:      "postgres-upgrade:latest",
		Registry:       "ghcr.io",
		ImageBase:      "flanksource/postgres-upgrade",
		SourceVersions: []string{"14"},
		TargetVersion:  "17",
		TestUser:       "testuser",
		TestPassword:   "testpass",
		TestDatabase:   "testdb",
	}

	upgradeTest := NewPostgresUpgradeTest(config)

	// Build the upgrade image
	t.Run("BuildUpgradeImage", func(t *testing.T) {
		if err := upgradeTest.buildUpgradeImage(); err != nil {
			t.Fatalf("Failed to build upgrade image: %v", err)
		}
	})

	// Test single upgrade path
	t.Run("Upgrade_14_to_17", func(t *testing.T) {
		if err := upgradeTest.testUpgrade("14", "17"); err != nil {
			t.Errorf("Upgrade from 14 to 17 failed: %v", err)
		}
	})

	// Cleanup
	t.Cleanup(func() {
		upgradeTest.cleanup()
	})
}

// TestShowUpgradeStatus shows the status of volumes and images
func TestShowUpgradeStatus(t *testing.T) {
	client := NewDockerClient(true)

	client.runner.Printf(colorBlue, colorBold, "📊 Docker Volumes:")

	versions := []string{"14", "15", "16", "17"}
	volumesFound := false

	for _, version := range versions {
		volumeName := fmt.Sprintf("pg%s-test-data", version)
		if _, err := GetVolume(volumeName); err == nil {
			client.runner.Printf(colorGray, "", "  ✅ %s", volumeName)
			volumesFound = true
		} else {
			client.runner.Printf(colorGray, "", "  ❌ %s (missing)", volumeName)
		}
	}

	if !volumesFound {
		client.runner.Printf(colorGray, "", "  No test volumes found")
	}

	client.runner.Printf(colorBlue, colorBold, "\n📊 Docker Images:")

	// Check for postgres-upgrade images
	result := client.runner.RunCommandQuiet("docker", "images", "--format", "{{.Repository}}:{{.Tag}}")
	if result.ExitCode == 0 {
		images := strings.Split(result.Stdout, "\n")
		upgradeImages := []string{}
		for _, img := range images {
			if strings.Contains(img, "postgres-upgrade") {
				upgradeImages = append(upgradeImages, img)
			}
		}

		if len(upgradeImages) > 0 {
			for _, img := range upgradeImages {
				client.runner.Printf(colorGray, "", "  ✅ %s", img)
			}
		} else {
			client.runner.Printf(colorGray, "", "  No postgres-upgrade images found")
		}
	}

	client.runner.Printf(colorBlue, colorBold, "\n📊 Running Containers:")

	// Check for running containers
	result = client.runner.RunCommandQuiet("docker", "ps", "--format", "{{.Names}}")
	if result.ExitCode == 0 {
		containers := strings.Split(strings.TrimSpace(result.Stdout), "\n")
		relatedContainers := []string{}
		for _, container := range containers {
			if strings.Contains(container, "postgres") || strings.Contains(container, "upgrade") {
				relatedContainers = append(relatedContainers, container)
			}
		}

		if len(relatedContainers) > 0 {
			for _, container := range relatedContainers {
				client.runner.Printf(colorGray, "", "  ✅ %s", container)
			}
		} else {
			client.runner.Printf(colorGray, "", "  No related containers running")
		}
	}
}
