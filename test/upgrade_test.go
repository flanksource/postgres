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

	// Use existing seeded volume (created by Taskfile seed tasks)
	sourceVolumeName := fmt.Sprintf("pg%s-test-data", fromVersion)
	
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
		Entrypoint: []string{"sh", "-c", "cp -a /source/. /target/"},
		Remove:     true,
	})
	if err != nil {
		targetVolume.Delete() // Clean up on failure
		return nil, fmt.Errorf("failed to copy volume data: %w", err)
	}

	// Wait for copy to complete
	if err := container.Wait(); err != nil {
		targetVolume.Delete() // Clean up on failure
		return nil, fmt.Errorf("volume copy failed: %w", err)
	}

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
if [ "$table_count" -lt 1 ]; then
    echo "ERROR: Tables missing. Expected at least 1, found $table_count"
    exit 1
fi
echo "✅ Tables are present ($table_count)"

# Check data integrity
record_count=$(psql -U postgres -d %s -t -c "SELECT COUNT(*) FROM test_upgrade;" | tr -d " ")
if [ "$record_count" != "5" ]; then
    echo "ERROR: Data integrity check failed. Expected 5 records, found $record_count"
    exit 1
fi
echo "✅ Data integrity verified (5 records)"

# Check that we can query the view
view_count=$(psql -U postgres -d %s -t -c "SELECT total_records FROM test_upgrade_summary;" | tr -d " ")
if [ "$view_count" != "5" ]; then
    echo "ERROR: View check failed. Expected 5 total_records, found $view_count"
    exit 1
fi
echo "✅ View test_upgrade_summary is working"

# Display sample data
echo "Sample data after upgrade:"
psql -U postgres -d %s -c "SELECT * FROM test_upgrade LIMIT 3;"

# Stop PostgreSQL
pg_ctl -D /var/lib/postgresql/data stop -w
`, toVersion, toVersion, toVersion, put.config.TestDatabase, put.config.TestDatabase,
			put.config.TestDatabase, put.config.TestDatabase))

	// Clean up container regardless of result
	defer put.client.runner.RunCommandQuiet("docker", "rm", "-f", containerName)

	if result.ExitCode != 0 {
		put.client.runner.Errorf("Verification STDOUT:\n%s", result.Stdout)
		put.client.runner.Errorf("Verification STDERR:\n%s", result.Stderr)
		return fmt.Errorf("verification failed with exit code %d", result.ExitCode)
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
		"View test_upgrade_summary is working",
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
