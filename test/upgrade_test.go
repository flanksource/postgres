package test

import (
	"fmt"
	"os"
	"strings"
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
	Extensions     []string // Extensions to test with upgrade
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

// buildUpgradeImage builds the PostgreSQL upgrade Docker image
func (put *PostgresUpgradeTest) buildUpgradeImage() error {
	put.client.runner.Printf(colorBlue, colorBold, "Building PostgreSQL upgrade image...")

	// Find the Dockerfile - check parent directory if not in current
	buildContext := "."
	if _, err := os.Stat("Dockerfile"); os.IsNotExist(err) {
		// Try parent directory
		if _, err := os.Stat("../Dockerfile"); os.IsNotExist(err) {
			return fmt.Errorf("Dockerfile not found in current or parent directory")
		}
		buildContext = ".."
	}

	result := put.client.runner.RunCommand("docker", "build", "-t", put.config.ImageName, buildContext)
	if result.ExitCode != 0 {
		return fmt.Errorf("failed to build image: %v", result.Err)
	}

	put.client.runner.Infof("Successfully built upgrade image")
	return nil
}

// testUpgrade tests a specific upgrade path
func (put *PostgresUpgradeTest) testUpgrade(fromVersion, toVersion string) error {
	put.client.runner.Printf(colorBlue, colorBold, "Testing upgrade from PostgreSQL %s to %s", fromVersion, toVersion)

	// Ensure source volume exists with seeded data
	sourceVolumeName := fmt.Sprintf("pg%s-test-data", fromVersion)
	if _, err := GetVolume(sourceVolumeName); err != nil {
		put.client.runner.Printf(colorYellow, "", "Source volume not found, creating and seeding...")
		opts := DefaultSeedOptions(fromVersion)
		if _, err := SeedPostgres(opts); err != nil {
			return fmt.Errorf("failed to seed source volume: %w", err)
		}
	}

	// Create a copy of the source volume for this test to avoid conflicts
	testVolumeName := fmt.Sprintf("pg%s-to-%s-test-%d", fromVersion, toVersion, time.Now().Unix())
	testVolume, err := put.copyVolume(sourceVolumeName, testVolumeName, fromVersion)
	if err != nil {
		return fmt.Errorf("failed to copy test volume: %w", err)
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

// copyVolume creates a copy of an existing volume for testing
func (put *PostgresUpgradeTest) copyVolume(sourceVolumeName, targetVolumeName, version string) (*Volume, error) {
	put.client.runner.Printf(colorGray, "", "📋 Copying volume %s to %s for testing...", sourceVolumeName, targetVolumeName)

	// Create target volume
	targetVolume, err := CreateVolume(VolumeOptions{
		Name: targetVolumeName,
		Labels: map[string]string{
			"postgres.version": version,
			"test":             "true",
			"source":           sourceVolumeName,
			"created":          time.Now().Format(time.RFC3339),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create target volume: %w", err)
	}

	// Copy data from source to target volume using a temporary container
	copyContainerName := fmt.Sprintf("volume-copy-%d", time.Now().Unix())
	container, err := Run(ContainerOptions{
		Name:  copyContainerName,
		Image: "alpine:latest",
		Volumes: map[string]string{
			sourceVolumeName: "/source:ro",
			targetVolumeName: "/target",
		},
		Command: []string{"sh", "-c", "cp -a /source/. /target/"},
		Detach:  true,
		Remove:  false, // We'll remove manually after waiting
	})
	if err != nil {
		targetVolume.Delete() // Clean up on failure
		return nil, fmt.Errorf("failed to copy volume data: %w", err)
	}

	// Wait for copy to complete
	if err := container.WaitFor(time.Minute); err != nil {
		container.Delete()
		targetVolume.Delete() // Clean up on failure
		return nil, fmt.Errorf("volume copy failed: %w", err)
	}

	// Clean up the copy container
	container.Delete()

	put.client.runner.Printf(colorGray, "", "✅ Volume copied successfully")
	return targetVolume, nil
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
		"-e", fmt.Sprintf("PG_VERSION=%s", toVersion),
		"-e", "AUTO_UPGRADE=true",
		"-e", "UPGRADE_ONLY=true",
		"-v", fmt.Sprintf("%s:/var/lib/postgresql/data", volume.Name),
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

// verifyUpgrade verifies that the upgrade was successful using SQL client
func (put *PostgresUpgradeTest) verifyUpgrade(volume *Volume, fromVersion, toVersion string) error {
	put.client.runner.Printf(colorGray, "", "🔍 Verifying upgrade from PostgreSQL %s to %s...", fromVersion, toVersion)

	containerName := fmt.Sprintf("verify-upgrade-%s-to-%s-%d", fromVersion, toVersion, time.Now().Unix())
	port := 5433 // Use non-standard port to avoid conflicts

	// Start PostgreSQL container with port mapping
	container, err := Run(ContainerOptions{
		Name:  containerName,
		Image: fmt.Sprintf("postgres:%s", toVersion),
		Env: map[string]string{
			"POSTGRES_PASSWORD": put.config.TestPassword,
		},
		Volumes: map[string]string{
			volume.Name: "/var/lib/postgresql/data",
		},
		Ports: map[string]string{
			fmt.Sprintf("%d", port): "5432",
		},
		Detach: true,
		Remove: false,
	})
	if err != nil {
		return fmt.Errorf("failed to start verification container: %w", err)
	}

	// Clean up container regardless of result
	defer func() {
		container.Stop()
		container.Delete()
	}()

	// Wait for PostgreSQL to be ready
	put.client.runner.Printf(colorGray, "", "Waiting for PostgreSQL to be ready...")
	if err := WaitForPostgres("localhost", port, "postgres", put.config.TestPassword, put.config.TestDatabase, time.Minute); err != nil {
		return fmt.Errorf("postgres did not become ready: %w", err)
	}

	// Create verifier and connect
	verifierConfig := UpgradeVerifierConfig{
		Host:     "localhost",
		Port:     port,
		User:     "postgres",
		Password: put.config.TestPassword,
		Database: put.config.TestDatabase,
		Timeout:  30 * time.Second,
	}

	verifier := NewUpgradeVerifier(verifierConfig)
	if err := verifier.Connect(); err != nil {
		return fmt.Errorf("failed to connect to postgres: %w", err)
	}
	defer verifier.Close()

	// Run verification
	if err := verifier.VerifyBasicUpgrade(toVersion, put.config.TestDatabase); err != nil {
		return fmt.Errorf("upgrade verification failed: %w", err)
	}

	put.client.runner.Infof("✅ Upgrade verification passed!")
	return nil
}

// cleanup removes temporary test volumes only (not base seed volumes)
func (put *PostgresUpgradeTest) cleanup() {
	put.client.runner.Printf(colorYellow, colorBold, "🧹 Cleaning up temporary test volumes...")

	// List all volumes and remove temporary test volumes only
	volumes, err := ListVolumes()
	if err != nil {
		put.client.runner.Printf(colorRed, "", "Failed to list volumes: %v", err)
		return
	}

	for _, volume := range volumes {
		// Only remove temporary test volumes (those with timestamp suffix like pg14-to-17-test-1234567890)
		// DO NOT remove base seed volumes (pg14-test-data, pg15-test-data, etc.) as they're reused
		if strings.HasPrefix(volume.Name, "pg") && strings.Contains(volume.Name, "-test-") &&
			!strings.HasSuffix(volume.Name, "-test-data") {
			put.client.runner.Printf(colorGray, "", "Removing temporary volume: %s", volume.Name)
			volume.Delete()
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

// testUpgradeWithExtensions tests PostgreSQL upgrade with extensions
func (put *PostgresUpgradeTest) testUpgradeWithExtensions(fromVersion, toVersion string) error {
	put.client.runner.Printf(colorBlue, colorBold, "Testing upgrade from PostgreSQL %s to %s with extensions", fromVersion, toVersion)

	// Create enhanced test volume with extensions
	sourceVolumeName := fmt.Sprintf("pg%s-enhanced-test-data", fromVersion)

	// Create and populate test volume with extensions
	if err := put.createTestVolumeWithExtensions(sourceVolumeName, fromVersion); err != nil {
		return fmt.Errorf("failed to create test volume with extensions: %w", err)
	}
	defer put.removeTestVolume(sourceVolumeName)

	// Create a copy for testing
	testVolumeName := fmt.Sprintf("pg%s-to-%s-extensions-test-%d", fromVersion, toVersion, time.Now().Unix())
	testVolume, err := put.copyVolume(sourceVolumeName, testVolumeName, fromVersion)
	if err != nil {
		return fmt.Errorf("failed to copy test volume: %w", err)
	}

	// Run the upgrade with enhanced image
	if err := put.runUpgradeWithExtensions(testVolume, fromVersion, toVersion); err != nil {
		put.client.runner.Errorf("❌ Upgrade with extensions failed, preserving volume %s for debugging", testVolume.Name)
		return fmt.Errorf("upgrade with extensions failed: %w", err)
	}

	// Verify the upgrade and extensions
	if err := put.verifyUpgradeWithExtensions(testVolume, fromVersion, toVersion); err != nil {
		put.client.runner.Errorf("❌ Extensions verification failed, preserving volume %s for debugging", testVolume.Name)
		return fmt.Errorf("extensions verification failed: %w", err)
	}

	// Clean up on success
	put.client.runner.Infof("✅ Upgrade from %s to %s with extensions successful", fromVersion, toVersion)
	testVolume.Delete()
	return nil
}

// createTestVolumeWithExtensions creates a test volume with extensions pre-installed
func (put *PostgresUpgradeTest) createTestVolumeWithExtensions(volumeName, version string) error {
	put.client.runner.Printf(colorGray, "", "📋 Creating test volume %s with extensions for PostgreSQL %s...", volumeName, version)

	// Create volume
	volume, err := CreateVolume(VolumeOptions{
		Name: volumeName,
		Labels: map[string]string{
			"postgres.version": version,
			"test":             "true",
			"extensions":       "true",
			"created":          time.Now().Format(time.RFC3339),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create volume: %w", err)
	}

	// Use enhanced image to initialize with extensions
	containerName := fmt.Sprintf("seed-extensions-%s-%d", version, time.Now().Unix())

	args := []string{
		"run", "--rm",
		"--name", containerName,
		"-v", fmt.Sprintf("%s:/var/lib/postgresql/data", volumeName),
		"-e", fmt.Sprintf("POSTGRES_PASSWORD=%s", put.config.TestPassword),
		"-e", fmt.Sprintf("POSTGRES_DB=%s", put.config.TestDatabase),
		"-e", fmt.Sprintf("POSTGRES_USER=%s", put.config.TestUser),
		"-e", fmt.Sprintf("POSTGRES_EXTENSIONS=%s", strings.Join(put.config.Extensions, ",")),
		"ghcr.io/flanksource/postgres:17-latest", // Use enhanced image
		"bash", "-c", fmt.Sprintf(`
			# Initialize PostgreSQL with extensions
			docker-entrypoint.sh postgres &
			PGPID=$!
			sleep 15

			# Create test data
			psql -U %s -d %s <<EOF
CREATE TABLE test_upgrade_extensions (
	id SERIAL PRIMARY KEY,
	data TEXT,
	vector_data VECTOR(3),
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO test_upgrade_extensions (data, vector_data) VALUES
('Test with extensions 1', '[1,2,3]'),
('Test with extensions 2', '[4,5,6]'),
('Test with extensions 3', '[7,8,9]'),
('Test with extensions 4', '[10,11,12]'),
('Test with extensions 5', '[13,14,15]');

-- Create an index using extension functionality
CREATE INDEX ON test_upgrade_extensions USING ivfflat (vector_data vector_cosine_ops) WITH (lists = 1);

-- Create a cron job (will be preserved through upgrade)
SELECT cron.schedule('cleanup-job', '0 0 * * *', 'DELETE FROM test_upgrade_extensions WHERE created_at < NOW() - INTERVAL ''30 days'';');

CREATE VIEW test_upgrade_extensions_summary AS
SELECT COUNT(*) as total_records, AVG(vector_data <-> '[0,0,0]') as avg_distance
FROM test_upgrade_extensions;
EOF

			# Stop PostgreSQL gracefully
			pg_ctl -D /var/lib/postgresql/data stop -m smart -w

			# Fix permissions
			chown -R $(id -u):$(id -g) /var/lib/postgresql/data
		`, put.config.TestUser, put.config.TestDatabase),
	}

	result := put.client.runner.RunCommand("docker", args...)
	if result.ExitCode != 0 {
		volume.Delete()
		return fmt.Errorf("failed to initialize volume with extensions: %v", result.Err)
	}

	put.client.runner.Printf(colorGray, "", "✅ Test volume with extensions created successfully")
	return nil
}

// runUpgradeWithExtensions runs upgrade using the enhanced image
func (put *PostgresUpgradeTest) runUpgradeWithExtensions(volume *Volume, fromVersion, toVersion string) error {
	put.client.runner.Statusf("🚀 Starting upgrade with extensions from PostgreSQL %s to %s...", fromVersion, toVersion)

	containerName := fmt.Sprintf("upgrade-extensions-%s-to-%s-%d", fromVersion, toVersion, time.Now().Unix())

	// Use enhanced image for upgrade
	args := []string{
		"run", "--rm",
		"--name", containerName,
		"-e", fmt.Sprintf("PG_VERSION=%s", toVersion),
		"-e", "AUTO_UPGRADE=true",
		"-e", fmt.Sprintf("POSTGRES_EXTENSIONS=%s", strings.Join(put.config.Extensions, ",")),
		"-v", fmt.Sprintf("%s:/var/lib/postgresql/data", volume.Name),
		"ghcr.io/flanksource/postgres:17-latest", // Use enhanced image
	}

	result := put.client.runner.RunCommand("docker", args...)

	if result.ExitCode != 0 {
		return fmt.Errorf("upgrade with extensions failed with exit code %d: %v", result.ExitCode, result.Stderr)
	}

	put.client.runner.Infof("✅ Upgrade with extensions completed")
	return nil
}

// verifyUpgradeWithExtensions verifies upgrade with extensions using SQL client
func (put *PostgresUpgradeTest) verifyUpgradeWithExtensions(volume *Volume, fromVersion, toVersion string) error {
	put.client.runner.Printf(colorGray, "", "🔍 Verifying upgrade with extensions from PostgreSQL %s to %s...", fromVersion, toVersion)

	containerName := fmt.Sprintf("verify-extensions-%s-to-%s-%d", fromVersion, toVersion, time.Now().Unix())
	port := 5434 // Use different port than basic verification to avoid conflicts

	// Start PostgreSQL container with port mapping
	container, err := Run(ContainerOptions{
		Name:  containerName,
		Image: fmt.Sprintf("postgres:%s", toVersion),
		Env: map[string]string{
			"POSTGRES_PASSWORD": put.config.TestPassword,
		},
		Volumes: map[string]string{
			volume.Name: "/var/lib/postgresql/data",
		},
		Ports: map[string]string{
			fmt.Sprintf("%d", port): "5432",
		},
		Detach: true,
		Remove: false,
	})
	if err != nil {
		return fmt.Errorf("failed to start verification container: %w", err)
	}

	// Clean up container regardless of result
	defer func() {
		container.Stop()
		container.Delete()
	}()

	// Wait for PostgreSQL to be ready
	put.client.runner.Printf(colorGray, "", "Waiting for PostgreSQL to be ready...")
	if err := WaitForPostgres("localhost", port, "postgres", put.config.TestPassword, put.config.TestDatabase, time.Minute); err != nil {
		return fmt.Errorf("postgres did not become ready: %w", err)
	}

	// Create verifier and connect
	verifierConfig := UpgradeVerifierConfig{
		Host:     "localhost",
		Port:     port,
		User:     "postgres",
		Password: put.config.TestPassword,
		Database: put.config.TestDatabase,
		Timeout:  30 * time.Second,
	}

	verifier := NewUpgradeVerifier(verifierConfig)
	if err := verifier.Connect(); err != nil {
		return fmt.Errorf("failed to connect to postgres: %w", err)
	}
	defer verifier.Close()

	// Run verification with extensions
	if err := verifier.VerifyUpgradeWithExtensions(toVersion, put.config.TestDatabase, put.config.Extensions); err != nil {
		return fmt.Errorf("upgrade with extensions verification failed: %w", err)
	}

	put.client.runner.Infof("✅ Upgrade with extensions verification passed!")
	return nil
}

// removeTestVolume removes a test volume
func (put *PostgresUpgradeTest) removeTestVolume(volumeName string) {
	if volume, err := GetVolume(volumeName); err == nil {
		volume.Delete()
	}
}
