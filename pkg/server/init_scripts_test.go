package server

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/flanksource/postgres/pkg"
)

func TestProcessInitFile(t *testing.T) {
	tmpDir := t.TempDir()

	sqlFile := filepath.Join(tmpDir, "01-test.sql")
	if err := os.WriteFile(sqlFile, []byte("SELECT 1;"), 0644); err != nil {
		t.Fatal(err)
	}

	gzFile := filepath.Join(tmpDir, "02-test.sql.gz")
	f, err := os.Create(gzFile)
	if err != nil {
		t.Fatal(err)
	}
	gzWriter := gzip.NewWriter(f)
	if _, err := gzWriter.Write([]byte("SELECT 2;")); err != nil {
		t.Fatal(err)
	}
	gzWriter.Close()
	f.Close()

	shFile := filepath.Join(tmpDir, "03-test.sh")
	if err := os.WriteFile(shFile, []byte("#!/bin/sh\necho 'test'"), 0755); err != nil {
		t.Fatal(err)
	}

	unknownFile := filepath.Join(tmpDir, "04-test.txt")
	if err := os.WriteFile(unknownFile, []byte("random content"), 0644); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 4 {
		t.Errorf("Expected 4 files, got %d", len(entries))
	}

	expectedFiles := map[string]string{
		"01-test.sql":    ".sql",
		"02-test.sql.gz": ".gz",
		"03-test.sh":     ".sh",
		"04-test.txt":    ".txt",
	}

	for _, entry := range entries {
		name := entry.Name()
		expectedExt, ok := expectedFiles[name]
		if !ok {
			t.Errorf("Unexpected file: %s", name)
			continue
		}

		actualExt := filepath.Ext(name)
		if actualExt != expectedExt {
			t.Errorf("File %s: expected ext %s, got %s", name, expectedExt, actualExt)
		}
	}
}

func TestProcessInitScriptsNonExistentDir(t *testing.T) {
	p := &Postgres{
		Config:  &pkg.PostgresConf{},
		DataDir: t.TempDir(),
	}

	err := p.ProcessInitScripts("/nonexistent/path")
	if err != nil {
		t.Errorf("Expected no error for nonexistent directory, got: %v", err)
	}
}

func TestProcessInitScriptsEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	p := &Postgres{
		Config:  &pkg.PostgresConf{},
		DataDir: tmpDir,
	}

	err := p.ProcessInitScripts(tmpDir)
	if err != nil {
		t.Errorf("Expected no error for empty directory, got: %v", err)
	}
}
