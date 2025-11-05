package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/flanksource/postgres/pkg/utils"
)

func TestSetupDockerEnv(t *testing.T) {
	os.Clearenv()

	os.Setenv("POSTGRES_USER", "testuser")
	os.Setenv("PGPASSWORD", "testpass")
	os.Setenv("POSTGRES_DB", "testdb")
	os.Setenv("POSTGRES_INITDB_ARGS", "--data-checksums")
	os.Setenv("POSTGRES_HOST_AUTH_METHOD", "md5")

	defer func() {
		os.Unsetenv("POSTGRES_USER")
		os.Unsetenv("PGPASSWORD")
		os.Unsetenv("POSTGRES_DB")
		os.Unsetenv("POSTGRES_INITDB_ARGS")
		os.Unsetenv("POSTGRES_HOST_AUTH_METHOD")
	}()

	env, err := SetupDockerEnv()
	if err != nil {
		t.Fatalf("SetupDockerEnv failed: %v", err)
	}

	if env.User != "testuser" {
		t.Errorf("Expected User=testuser, got %s", env.User)
	}

	if string(env.Password) != "testpass" {
		t.Errorf("Expected Password=testpass, got %s", env.Password)
	}

	if env.Database != "testdb" {
		t.Errorf("Expected Database=testdb, got %s", env.Database)
	}

	if env.InitDBArgs != "--data-checksums" {
		t.Errorf("Expected InitDBArgs=--data-checksums, got %s", env.InitDBArgs)
	}

	if env.HostAuthMethod != "md5" {
		t.Errorf("Expected HostAuthMethod=md5, got %s", env.HostAuthMethod)
	}
}

func TestSetupDockerEnvDefaults(t *testing.T) {
	os.Clearenv()

	env, err := SetupDockerEnv()
	if err != nil {
		t.Fatalf("SetupDockerEnv failed: %v", err)
	}

	if env.User != "postgres" {
		t.Errorf("Expected default User=postgres, got %s", env.User)
	}

	if env.Database != "postgres" {
		t.Errorf("Expected default Database=postgres, got %s", env.Database)
	}
}

func TestValidateMinimumEnvNoPassword(t *testing.T) {
	env := &DockerEnv{
		Password:       "",
		HostAuthMethod: "",
	}

	err := ValidateMinimumEnv(env, false)
	if err == nil {
		t.Error("Expected error when password is empty and auth method is not trust")
	}
}

func TestValidateMinimumEnvWithTrust(t *testing.T) {
	env := &DockerEnv{
		Password:       "",
		HostAuthMethod: "trust",
	}

	err := ValidateMinimumEnv(env, false)
	if err != nil {
		t.Errorf("Expected no error with trust auth method, got: %v", err)
	}
}

func TestValidateMinimumEnvWithPassword(t *testing.T) {
	env := &DockerEnv{
		Password:       utils.SensitiveString("mypassword"),
		HostAuthMethod: "",
	}

	err := ValidateMinimumEnv(env, false)
	if err != nil {
		t.Errorf("Expected no error with password set, got: %v", err)
	}
}

func TestValidateMinimumEnvExistingDatabase(t *testing.T) {
	env := &DockerEnv{
		Password:       "",
		HostAuthMethod: "",
	}

	err := ValidateMinimumEnv(env, true)
	if err != nil {
		t.Errorf("Expected no error for existing database, got: %v", err)
	}
}

func TestDatabaseAlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()

	if DatabaseAlreadyExists(tmpDir) {
		t.Error("Expected false for directory without PG_VERSION")
	}

	versionFile := filepath.Join(tmpDir, "PG_VERSION")
	if err := os.WriteFile(versionFile, []byte("17\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if !DatabaseAlreadyExists(tmpDir) {
		t.Error("Expected true for directory with PG_VERSION")
	}
}
