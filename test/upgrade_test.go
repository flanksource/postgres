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

	// Create and seed source volume
	sourceVolume, err := put.createAndSeedVolume(fromVersion)
	if err != nil {
		return fmt.Errorf("failed to create and seed source volume: %w", err)
	}
	defer sourceVolume.Delete()

	// Clone volume for upgrade test
	testVolumeName := fmt.Sprintf("pg%s-to-%s-test-%d", fromVersion, toVersion, time.Now().Unix())
	testVolume, err := sourceVolume.CloneVolume(testVolumeName)
	if err != nil {
		return fmt.Errorf("failed to clone volume: %w", err)
	}
	defer testVolume.Delete()

	// Run the upgrade
	if err := put.runUpgrade(testVolume, fromVersion, toVersion); err != nil {
		return fmt.Errorf("upgrade failed: %w", err)
	}

	// Verify the upgrade
	if err := put.verifyUpgrade(testVolume, fromVersion, toVersion); err != nil {
		return fmt.Errorf("upgrade verification failed: %w", err)
	}

	put.client.runner.Infof("✅ Upgrade from %s to %s successful", fromVersion, toVersion)
	return nil
}

// createAndSeedVolume creates a volume and seeds it with test data
func (put *PostgresUpgradeTest) createAndSeedVolume(version string) (*Volume, error) {
	volumeName := fmt.Sprintf("pg%s-test-data", version)

	// Check if volume already exists
	volume, err := GetVolume(volumeName)
	if err == nil {
		put.client.runner.Printf(colorGray, "", "Volume %s already exists, verifying seed data...", volumeName)
		if err := put.verifySeedData(volume, version); err == nil {
			return volume, nil
		}
		// If verification fails, recreate the volume
		volume.Delete()
	}

	// Create new volume
	volume, err = CreateVolume(VolumeOptions{
		Name: volumeName,
		Labels: map[string]string{
			"postgres.version": version,
			"test":             "true",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create volume: %w", err)
	}

	// Seed the volume
	if err := put.seedVolume(volume, version); err != nil {
		volume.Delete()
		return nil, fmt.Errorf("failed to seed volume: %w", err)
	}

	return volume, nil
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
		Remove: true,
	})
	if err != nil {
		return fmt.Errorf("failed to start PostgreSQL container: %w", err)
	}
	defer container.Delete()

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
		return fmt.Errorf("seed verification failed: %w", err)
	}

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
	container, err := Run(ContainerOptions{
		Name:  fmt.Sprintf("upgrade-%s-to-%s-%d", fromVersion, toVersion, time.Now().Unix()),
		Image: imageName,
		User:  "postgres",
		Env: map[string]string{
			"TARGET_VERSION": toVersion,
		},
		Volumes: map[string]string{
			volume.Name: "/var/lib/postgresql/data",
		},
		WorkingDir: "/var/lib/postgresql",
		Remove:     true,
	})
	if err != nil {
		return fmt.Errorf("failed to run upgrade container: %w", err)
	}

	// The upgrade process should complete and exit
	// Wait a bit to ensure it's done
	time.Sleep(10 * time.Second)

	// Check logs for any errors
	logs, err := container.Logs()
	if err != nil {
		// Container might have already been removed (which is expected)
		put.client.runner.Printf(colorGray, "", "Upgrade container completed and was removed")
	} else {
		// Check for error indicators in logs
		if strings.Contains(logs, "ERROR") || strings.Contains(logs, "FATAL") {
			return fmt.Errorf("upgrade process reported errors: %s", logs)
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
	// Skip if not in CI or explicitly enabled
	if os.Getenv("RUN_UPGRADE_TESTS") != "true" {
		t.Skip("Skipping upgrade tests. Set RUN_UPGRADE_TESTS=true to run")
	}

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
