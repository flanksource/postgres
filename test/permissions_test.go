package test

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestPostgresUserPermissions tests that the container runs as postgres user by default
func TestPostgresUserPermissions(t *testing.T) {
	client := NewDockerClient(true)

	testCases := []struct {
		name          string
		user          string // Empty string means default user (postgres)
		shouldSucceed bool
		checkUID      string // Expected UID in the container
	}{
		{
			name:          "Default user (postgres)",
			user:          "",
			shouldSucceed: true,
			checkUID:      "999",
		},
		{
			name:          "Explicit postgres user",
			user:          "postgres",
			shouldSucceed: true,
			checkUID:      "999",
		},
		{
			name:          "Explicit UID 999",
			user:          "999",
			shouldSucceed: true,
			checkUID:      "999",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			volumeName := fmt.Sprintf("pgdata-permissions-test-%d", time.Now().Unix())
			containerName := fmt.Sprintf("postgres-permissions-test-%d", time.Now().Unix())

			// Clean up after test
			defer func() {
				client.StopContainer(containerName)
				client.RemoveVolume(volumeName)
			}()

			// Build image first
			imageName := "flanksource/postgres:test"
			t.Logf("Building image: %s", imageName)
			if err := client.BuildImage(".", imageName); err != nil {
				t.Fatalf("Failed to build image: %v", err)
			}

			// Create named volume
			t.Logf("Creating volume: %s", volumeName)
			if err := client.CreateVolume(volumeName); err != nil {
				t.Fatalf("Failed to create volume: %v", err)
			}

			// Start container with optional user override
			env := map[string]string{
				"POSTGRES_PASSWORD": "testpass",
			}

			runOpts := []string{
				"-v", fmt.Sprintf("%s:/var/lib/postgresql/data", volumeName),
			}

			if tc.user != "" {
				runOpts = append(runOpts, "--user", tc.user)
			}

			t.Logf("Starting container: %s (user: %s)", containerName, tc.user)
			if err := client.RunContainer(imageName, containerName, env, runOpts...); err != nil {
				if tc.shouldSucceed {
					t.Fatalf("Failed to start container: %v", err)
				} else {
					t.Logf("Container failed to start as expected: %v", err)
					return
				}
			}

			// Wait for PostgreSQL to be ready
			t.Log("Waiting for PostgreSQL to be ready...")
			if err := client.WaitForPostgres(containerName, "postgres", "testpass", 60*time.Second); err != nil {
				if tc.shouldSucceed {
					t.Fatalf("PostgreSQL failed to become ready: %v", err)
				} else {
					t.Logf("PostgreSQL failed as expected: %v", err)
					return
				}
			}

			// Check the UID of the running process
			t.Log("Checking process UID...")
			output, err := client.ExecInContainer(containerName, []string{"id", "-u"})
			if err != nil {
				t.Fatalf("Failed to check UID: %v", err)
			}

			actualUID := strings.TrimSpace(output)
			if actualUID != tc.checkUID {
				t.Errorf("Expected UID %s, got %s", tc.checkUID, actualUID)
			} else {
				t.Logf("✅ Process running as UID %s", actualUID)
			}

			// Verify PGDATA ownership
			t.Log("Checking PGDATA ownership...")
			output, err = client.ExecInContainer(containerName, []string{"stat", "-c", "%u:%g", "/var/lib/postgresql/data"})
			if err != nil {
				t.Fatalf("Failed to check PGDATA ownership: %v", err)
			}

			ownership := strings.TrimSpace(output)
			expectedOwnership := "999:999"
			if ownership != expectedOwnership {
				t.Errorf("Expected PGDATA ownership %s, got %s", expectedOwnership, ownership)
			} else {
				t.Logf("✅ PGDATA owned by %s", ownership)
			}

			// Verify PostgreSQL is accessible
			t.Log("Testing database connectivity...")
			queryOutput, err := client.ExecInContainer(containerName, []string{
				"psql", "-U", "postgres", "-c", "SELECT version();",
			})
			if err != nil {
				t.Fatalf("Failed to query database: %v", err)
			}

			if !strings.Contains(queryOutput, "PostgreSQL") {
				t.Errorf("Unexpected query output: %s", queryOutput)
			} else {
				t.Log("✅ Database is accessible")
			}
		})
	}
}

// TestPostgresInitAsPostgresUser tests that PGDATA initialization works as postgres user
func TestPostgresInitAsPostgresUser(t *testing.T) {
	client := NewDockerClient(true)

	volumeName := fmt.Sprintf("pgdata-init-test-%d", time.Now().Unix())
	containerName := fmt.Sprintf("postgres-init-test-%d", time.Now().Unix())

	// Clean up after test
	defer func() {
		client.StopContainer(containerName)
		client.RemoveVolume(volumeName)
	}()

	// Build image
	imageName := "flanksource/postgres:test"
	t.Logf("Building image: %s", imageName)
	if err := client.BuildImage(".", imageName); err != nil {
		t.Fatalf("Failed to build image: %v", err)
	}

	// Create empty volume
	t.Logf("Creating empty volume: %s", volumeName)
	if err := client.CreateVolume(volumeName); err != nil {
		t.Fatalf("Failed to create volume: %v", err)
	}

	// Start container (should initialize PGDATA as postgres user)
	env := map[string]string{
		"POSTGRES_PASSWORD": "testpass",
	}

	runOpts := []string{
		"-v", fmt.Sprintf("%s:/var/lib/postgresql/data", volumeName),
	}

	t.Logf("Starting container for first time init: %s", containerName)
	if err := client.RunContainer(imageName, containerName, env, runOpts...); err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	// Wait for PostgreSQL to be ready
	t.Log("Waiting for PostgreSQL to initialize and be ready...")
	if err := client.WaitForPostgres(containerName, "postgres", "testpass", 120*time.Second); err != nil {
		t.Fatalf("PostgreSQL failed to become ready: %v", err)
	}

	// Verify initialization was done by postgres user
	t.Log("Checking PGDATA file ownership...")
	output, err := client.ExecInContainer(containerName, []string{
		"find", "/var/lib/postgresql/data", "-maxdepth", "1", "-type", "f", "-exec", "stat", "-c", "%u:%g:%n", "{}", ";",
	})
	if err != nil {
		t.Fatalf("Failed to check file ownership: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if !strings.HasPrefix(line, "999:999:") {
			t.Errorf("File not owned by postgres user: %s", line)
		}
	}

	t.Log("✅ All files in PGDATA owned by postgres user (999:999)")

	// Verify database works
	t.Log("Creating test table...")
	_, err = client.ExecInContainer(containerName, []string{
		"psql", "-U", "postgres", "-c", "CREATE TABLE test_table (id SERIAL PRIMARY KEY, name TEXT);",
	})
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	_, err = client.ExecInContainer(containerName, []string{
		"psql", "-U", "postgres", "-c", "INSERT INTO test_table (name) VALUES ('test');",
	})
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	output, err = client.ExecInContainer(containerName, []string{
		"psql", "-U", "postgres", "-t", "-c", "SELECT COUNT(*) FROM test_table;",
	})
	if err != nil {
		t.Fatalf("Failed to query test table: %v", err)
	}

	count := strings.TrimSpace(output)
	if count != "1" {
		t.Errorf("Expected 1 row, got: %s", count)
	}

	t.Log("✅ Database operations work correctly")
}

// TestPostgresDryRun tests the --dry-run flag for permission validation
func TestPostgresDryRun(t *testing.T) {
	client := NewDockerClient(true)

	volumeName := fmt.Sprintf("pgdata-dryrun-test-%d", time.Now().Unix())
	containerName := fmt.Sprintf("postgres-dryrun-test-%d", time.Now().Unix())

	// Clean up after test
	defer func() {
		client.RemoveVolume(volumeName)
	}()

	// Build image
	imageName := "flanksource/postgres:test"
	t.Logf("Building image: %s", imageName)
	if err := client.BuildImage(".", imageName); err != nil {
		t.Fatalf("Failed to build image: %v", err)
	}

	// Create volume with postgres ownership
	t.Logf("Creating volume: %s", volumeName)
	if err := client.CreateVolume(volumeName); err != nil {
		t.Fatalf("Failed to create volume: %v", err)
	}

	// Initialize the volume first
	initContainer := containerName + "-init"
	env := map[string]string{
		"POSTGRES_PASSWORD": "testpass",
	}

	runOpts := []string{
		"-v", fmt.Sprintf("%s:/var/lib/postgresql/data", volumeName),
	}

	t.Log("Initializing volume...")
	if err := client.RunContainer(imageName, initContainer, env, runOpts...); err != nil {
		t.Fatalf("Failed to start init container: %v", err)
	}

	if err := client.WaitForPostgres(initContainer, "postgres", "testpass", 60*time.Second); err != nil {
		t.Fatalf("PostgreSQL failed to initialize: %v", err)
	}

	client.StopContainer(initContainer)

	// Now test dry-run
	t.Log("Testing dry-run mode...")
	dryRunContainer := containerName + "-dryrun"

	// Override entrypoint to run postgres-cli with --dry-run
	runOpts = append(runOpts, "--entrypoint", "postgres-cli")

	if err := client.RunContainerWithCommand(imageName, dryRunContainer, env, []string{
		"auto-start", "--dry-run", "--data-dir", "/var/lib/postgresql/data",
	}, runOpts...); err != nil {
		t.Fatalf("Failed to run dry-run: %v", err)
	}

	// Wait a bit for command to complete
	time.Sleep(3 * time.Second)

	// Check logs for dry-run output
	logs, err := client.GetContainerLogs(dryRunContainer)
	if err != nil {
		t.Fatalf("Failed to get container logs: %v", err)
	}

	t.Logf("Dry-run output:\n%s", logs)

	if !strings.Contains(logs, "Permission checks passed") {
		t.Error("Expected 'Permission checks passed' in dry-run output")
	}

	if !strings.Contains(logs, "Dry-run validation completed successfully") {
		t.Error("Expected 'Dry-run validation completed successfully' in dry-run output")
	}

	client.StopContainer(dryRunContainer)

	t.Log("✅ Dry-run validation works correctly")
}
