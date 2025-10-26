package test

import (
	_ "embed"
	"fmt"
	"time"
)

//go:embed sql/seed-test-data.sql
var seedSQL string

// SeedOptions provides configuration for seeding a PostgreSQL version
type SeedOptions struct {
	Version      string
	Password     string
	Database     string
	User         string
	VolumeName   string // Optional: specify custom volume name
	ReuseVolume  bool   // If true, reuse existing volume if it exists
}

// DefaultSeedOptions returns default seeding options for a given version
func DefaultSeedOptions(version string) SeedOptions {
	return SeedOptions{
		Version:     version,
		Password:    "testpass",
		Database:    "testdb",
		User:        "postgres",
		VolumeName:  fmt.Sprintf("pg%s-test-data", version),
		ReuseVolume: true,
	}
}

// SeedPostgres creates and seeds a PostgreSQL test database for the specified version
func SeedPostgres(opts SeedOptions) (*Volume, error) {
	client := NewDockerClient(true)
	client.runner.Printf(colorBlue, colorBold, "Seeding PostgreSQL %s test data...", opts.Version)

	// Check if volume already exists and reuse is enabled
	if opts.ReuseVolume {
		if existingVolume, err := GetVolume(opts.VolumeName); err == nil {
			client.runner.Printf(colorGray, "", "✅ Reusing existing volume: %s", opts.VolumeName)
			return existingVolume, nil
		}
	}

	// Create volume
	volume, err := CreateVolume(VolumeOptions{
		Name: opts.VolumeName,
		Labels: map[string]string{
			"postgres.version": opts.Version,
			"test":             "true",
			"seeded":           "true",
			"created":          time.Now().Format(time.RFC3339),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create volume: %w", err)
	}

	// Run postgres container to initialize and seed the database
	containerName := fmt.Sprintf("seed-pg%s-%d", opts.Version, time.Now().Unix())

	// Create a temporary file with the seed SQL
	container, err := Run(ContainerOptions{
		Name:  containerName,
		Image: fmt.Sprintf("postgres:%s-bookworm", opts.Version),
		Env: map[string]string{
			"POSTGRES_PASSWORD": opts.Password,
			"POSTGRES_DB":       opts.Database,
			"POSTGRES_USER":     opts.User,
		},
		Volumes: map[string]string{
			opts.VolumeName: "/var/lib/postgresql/data",
		},
		Remove: false, // We'll remove manually after initialization
		Detach: true,
	})
	if err != nil {
		volume.Delete() // Clean up volume on failure
		return nil, fmt.Errorf("failed to start postgres container: %w", err)
	}

	// Wait for PostgreSQL to be ready
	client.runner.Printf(colorGray, "", "Waiting for PostgreSQL to initialize...")
	time.Sleep(10 * time.Second)

	// Execute the seed SQL
	client.runner.Printf(colorGray, "", "Executing seed SQL...")
	_, err = container.Exec("psql", "-U", opts.User, "-d", opts.Database, "-c", seedSQL)
	if err != nil {
		container.Stop()
		container.Delete()
		volume.Delete()
		return nil, fmt.Errorf("failed to execute seed SQL: %w", err)
	}

	// Stop container (PostgreSQL will stop automatically)
	client.runner.Printf(colorGray, "", "Stopping container...")
	container.Stop()
	container.Delete()

	client.runner.Printf(colorGreen, colorBold, "✅ Successfully seeded PostgreSQL %s", opts.Version)
	return volume, nil
}

// SeedAllVersions seeds all PostgreSQL versions for testing
func SeedAllVersions() error {
	versions := []string{"14", "15", "16", "17"}

	for _, version := range versions {
		opts := DefaultSeedOptions(version)
		if _, err := SeedPostgres(opts); err != nil {
			return fmt.Errorf("failed to seed PostgreSQL %s: %w", version, err)
		}
	}

	return nil
}

// CleanupSeedVolumes removes all test seed volumes
func CleanupSeedVolumes() error {
	client := NewDockerClient(true)
	client.runner.Printf(colorYellow, colorBold, "Cleaning up seed volumes...")

	volumes, err := ListVolumes()
	if err != nil {
		return fmt.Errorf("failed to list volumes: %w", err)
	}

	for _, volume := range volumes {
		// Remove PostgreSQL test seed volumes (pg14-test-data, pg15-test-data, etc.)
		if volume.Name == "pg14-test-data" || volume.Name == "pg15-test-data" ||
		   volume.Name == "pg16-test-data" || volume.Name == "pg17-test-data" {
			client.runner.Printf(colorGray, "", "Removing seed volume: %s", volume.Name)
			if err := volume.Delete(); err != nil {
				client.runner.Printf(colorRed, "", "Failed to delete %s: %v", volume.Name, err)
			}
		}
	}

	client.runner.Printf(colorGreen, colorBold, "✅ Cleanup completed")
	return nil
}
