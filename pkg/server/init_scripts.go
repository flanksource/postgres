package server

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/flanksource/clicky"
	"github.com/flanksource/commons/logger"
)

// ProcessInitScripts processes initialization scripts from /docker-entrypoint-initdb.d/
// Supports .sh, .sql, .sql.gz, .sql.xz, .sql.zst files
func (p *Postgres) ProcessInitScripts(initDir string) error {
	if _, err := os.Stat(initDir); os.IsNotExist(err) {
		logger.Debugf("Init directory %s does not exist, skipping init scripts", initDir)
		return nil
	}

	entries, err := os.ReadDir(initDir)
	if err != nil {
		return fmt.Errorf("failed to read init directory %s: %w", initDir, err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, filepath.Join(initDir, entry.Name()))
		}
	}

	sort.Strings(files)

	if len(files) == 0 {
		logger.Debugf("No init scripts found in %s", initDir)
		return nil
	}

	logger.Infof("Processing %d initialization scripts from %s", len(files), initDir)

	for _, file := range files {
		if err := p.processInitFile(file); err != nil {
			return fmt.Errorf("failed to process init file %s: %w", file, err)
		}
	}

	logger.Infof("Successfully processed all initialization scripts")
	return nil
}

func (p *Postgres) processInitFile(file string) error {
	ext := filepath.Ext(file)
	baseName := filepath.Base(file)

	logger.Infof("Processing init file: %s", baseName)

	switch {
	case ext == ".sh":
		return p.processShellScript(file)
	case ext == ".sql":
		return p.processSQLFile(file, false)
	case ext == ".gz" && strings.HasSuffix(file, ".sql.gz"):
		return p.processSQLFile(file, true)
	case ext == ".xz" && strings.HasSuffix(file, ".sql.xz"):
		return p.processCompressedSQL(file, "xz")
	case ext == ".zst" && strings.HasSuffix(file, ".sql.zst"):
		return p.processCompressedSQL(file, "zstd")
	default:
		logger.Warnf("Skipping unsupported file type: %s", baseName)
		return nil
	}
}

func (p *Postgres) processShellScript(file string) error {
	info, err := os.Stat(file)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	if info.Mode()&0111 != 0 {
		cmd := clicky.Exec(file)
		cmd.Env = map[string]string{
			"PGDATA":     p.DataDir,
			"PGHOST":     p.Host,
			"PGPORT":     fmt.Sprintf("%d", p.Port),
			"PGUSER":     p.Username,
			"PGDATABASE": p.Database,
		}
		if p.Password != "" {
			cmd.Env["PGPASSWORD"] = string(p.Password)
		}

		process := cmd.Run()
		if process.Err != nil {
			return fmt.Errorf("shell script failed: %w\nStdout: %s\nStderr: %s",
				process.Err, process.GetStdout(), process.GetStderr())
		}
		return nil
	}

	return fmt.Errorf("shell script is not executable: %s", file)
}

func (p *Postgres) processSQLFile(file string, compressed bool) error {
	var content []byte
	var err error

	if compressed {
		f, err := os.Open(file)
		if err != nil {
			return fmt.Errorf("failed to open compressed file: %w", err)
		}
		defer f.Close()

		gzReader, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()

		content, err = io.ReadAll(gzReader)
		if err != nil {
			return fmt.Errorf("failed to read compressed content: %w", err)
		}
	} else {
		content, err = os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read SQL file: %w", err)
		}
	}

	return p.executeSQLContent(string(content), filepath.Base(file))
}

func (p *Postgres) processCompressedSQL(file, compressor string) error {
	var cmd *exec.Cmd

	switch compressor {
	case "xz":
		cmd = exec.Command("xz", "-dc", file)
	case "zstd":
		cmd = exec.Command("zstd", "-dc", file)
	default:
		return fmt.Errorf("unsupported compressor: %s", compressor)
	}

	content, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to decompress file with %s: %w", compressor, err)
	}

	return p.executeSQLContent(string(content), filepath.Base(file))
}

func (p *Postgres) executeSQLContent(sql, filename string) error {
	psqlPath := filepath.Join(p.BinDir, "psql")

	cmd := clicky.Exec(psqlPath,
		"-h", p.Host,
		"-p", fmt.Sprintf("%d", p.Port),
		"-U", p.Username,
		"-d", p.Database,
		"-v", "ON_ERROR_STOP=1",
		"-c", sql,
	)

	if p.Password != "" {
		cmd.Env = map[string]string{"PGPASSWORD": string(p.Password)}
	}

	process := cmd.Run()
	if process.Err != nil {
		return fmt.Errorf("SQL execution failed for %s: %w\nStdout: %s\nStderr: %s",
			filename, process.Err, process.GetStdout(), process.GetStderr())
	}

	logger.Debugf("Successfully executed SQL from %s", filename)
	return nil
}
