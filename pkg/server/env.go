package server

import (
	"fmt"
	"os"

	"github.com/flanksource/commons/logger"
	"github.com/flanksource/postgres/pkg/utils"
)

type DockerEnv struct {
	User           string
	Password       utils.SensitiveString
	Database       string
	InitDBArgs     string
	WALDir         string
	HostAuthMethod string
}

// SetupDockerEnv loads Docker-compatible environment variables
func SetupDockerEnv() (*DockerEnv, error) {
	env := &DockerEnv{}

	var err error
	userStr, err := utils.FileEnv("POSTGRES_USER", "postgres")
	if err != nil {
		return nil, err
	}
	env.User = userStr

	passwordStr, err := utils.FileEnv("POSTGRES_PASSWORD", "")
	if err != nil {
		return nil, err
	}
	env.Password = utils.SensitiveString(passwordStr)

	env.Database, err = utils.FileEnv("POSTGRES_DB", env.User)
	if err != nil {
		return nil, err
	}

	env.InitDBArgs, err = utils.FileEnv("POSTGRES_INITDB_ARGS", "")
	if err != nil {
		return nil, err
	}

	env.WALDir, err = utils.FileEnv("POSTGRES_INITDB_WALDIR", "")
	if err != nil {
		return nil, err
	}

	env.HostAuthMethod, err = utils.FileEnv("POSTGRES_HOST_AUTH_METHOD", "")
	if err != nil {
		return nil, err
	}

	return env, nil
}

// ValidateMinimumEnv validates that required environment variables are set
func ValidateMinimumEnv(env *DockerEnv, databaseExists bool) error {
	if databaseExists {
		logger.Debugf("Database already exists, skipping initialization validation")
		return nil
	}

	if env.Password.IsEmpty() && env.HostAuthMethod != "trust" {
		return fmt.Errorf("POSTGRES_PASSWORD or POSTGRES_PASSWORD_FILE is required unless POSTGRES_HOST_AUTH_METHOD=trust")
	}

	if !env.Password.IsEmpty() {
		passwordLen := len(string(env.Password))
		if passwordLen > 100 {
			logger.Warnf("WARNING: Password is %d characters long", passwordLen)
			logger.Warnf("PostgreSQL 13+ has a bug with passwords longer than 100 characters")
			logger.Warnf("See: https://www.postgresql.org/message-id/16735-d17ae6ae6e5f7d9d%%40postgresql.org")
		}
	}

	if env.HostAuthMethod == "trust" {
		logger.Warnf("WARNING: POSTGRES_HOST_AUTH_METHOD=trust is set!")
		logger.Warnf("This allows anyone to connect to the database without a password!")
		logger.Warnf("This is NOT recommended for production use!")
	}

	return nil
}

// DatabaseAlreadyExists checks if the database has already been initialized
func DatabaseAlreadyExists(dataDir string) bool {
	_, err := os.Stat(fmt.Sprintf("%s/PG_VERSION", dataDir))
	return err == nil
}
