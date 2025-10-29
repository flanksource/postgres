package server

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/exec"
	"github.com/flanksource/commons/logger"
)

type TempServerOptions struct {
	UnixSocketOnly bool
	Port           int
}

// StartTempServer starts a temporary PostgreSQL server for initialization tasks
// If UnixSocketOnly is true, uses listen_addresses=” for Unix socket-only mode
func (p *Postgres) StartTempServer(opts TempServerOptions) (*exec.Process, error) {
	if p.BinDir == "" {
		return nil, fmt.Errorf("BinDir not specified")
	}

	if p.DataDir == "" {
		return nil, fmt.Errorf("DataDir not specified")
	}

	pgCtlPath := filepath.Join(p.BinDir, "pg_ctl")

	args := []string{
		"-D", p.DataDir,
		"-w",
		"start",
	}

	var pgOptions []string
	if opts.UnixSocketOnly {
		pgOptions = append(pgOptions, "-c", "listen_addresses=''")
	} else if opts.Port > 0 {
		pgOptions = append(pgOptions, "-c", fmt.Sprintf("port=%d", opts.Port))
	}

	if len(pgOptions) > 0 {
		args = append(args, "-o")
		optString := ""
		for i, opt := range pgOptions {
			if i > 0 {
				optString += " "
			}
			optString += opt
		}
		args = append(args, optString)
	}

	env := map[string]string{
		"PGDATA": p.DataDir,
	}

	// Unset NOTIFY_SOCKET to avoid systemd interference
	if _, exists := os.LookupEnv("NOTIFY_SOCKET"); exists {
		env["NOTIFY_SOCKET"] = ""
	}

	cmd := clicky.Exec(pgCtlPath, args...)
	cmd.Env = env

	process := cmd.Run()
	if process.Err != nil {
		return nil, fmt.Errorf("failed to start temp server: %w\nStdout: %s\nStderr: %s",
			process.Err, process.GetStdout(), process.GetStderr())
	}

	logger.Infof("Temporary PostgreSQL server started")
	return process, nil
}

// StopTempServer stops a temporary PostgreSQL server
func (p *Postgres) StopTempServer() error {
	if p.BinDir == "" {
		return fmt.Errorf("BinDir not specified")
	}

	if p.DataDir == "" {
		return fmt.Errorf("DataDir not specified")
	}

	pgCtlPath := filepath.Join(p.BinDir, "pg_ctl")

	args := []string{
		"-D", p.DataDir,
		"-m", "fast",
		"-w",
		"stop",
	}

	process := clicky.Exec(pgCtlPath, args...).Run()
	if process.Err != nil {
		return fmt.Errorf("failed to stop temp server: %w\nStdout: %s\nStderr: %s",
			process.Err, process.GetStdout(), process.GetStderr())
	}

	logger.Infof("Temporary PostgreSQL server stopped")
	time.Sleep(1 * time.Second)
	return nil
}
