package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanDataDirFiles(t *testing.T) {
	tmpDir := t.TempDir()

	testFiles := []struct {
		name    string
		content string
	}{
		{"postgresql.conf", "# PostgreSQL configuration\nport = 5432\n"},
		{"pg_hba.conf", "# Host-based authentication\nlocal all all trust\n"},
		{"postmaster.opts", "--port=5432\n"},
		{"postmaster.pid", "12345\n"},
		{"postgresql.tune.conf", "# Tuning parameters\nshared_buffers = 256MB\n"},
	}

	for _, tf := range testFiles {
		path := filepath.Join(tmpDir, tf.name)
		if err := os.WriteFile(path, []byte(tf.content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", tf.name, err)
		}
	}

	tree, err := scanDataDirFiles(tmpDir)
	if err != nil {
		t.Fatalf("scanDataDirFiles failed: %v", err)
	}

	if tree == nil {
		t.Fatal("Expected non-nil tree")
	}

	if tree.Label != "Data Directory" {
		t.Errorf("Expected label 'Data Directory', got %s", tree.Label)
	}

	if tree.Path != tmpDir {
		t.Errorf("Expected path %s, got %s", tmpDir, tree.Path)
	}

	if len(tree.Files) != 5 {
		t.Errorf("Expected 5 files, got %d", len(tree.Files))
	}

	for _, file := range tree.Files {
		if file.Name == "" {
			t.Error("File name should not be empty")
		}
		if file.Size == 0 {
			t.Errorf("File %s should have non-zero size", file.Name)
		}
		if file.MD5 == "" || file.MD5 == "error" {
			t.Errorf("File %s should have valid MD5 hash", file.Name)
		}
		if file.Modified.IsZero() {
			t.Errorf("File %s should have modified time", file.Name)
		}
	}
}

func TestDataDirTreeTreeNode(t *testing.T) {
	tree := &DataDirTree{
		Label: "Test Directory",
		Path:  "/test/path",
		Files: []DataDirFileInfo{
			{
				Name:     "test.conf",
				Path:     "/test/path/test.conf",
				Size:     1024,
				MD5:      "abc12345",
				Modified: time.Now(),
			},
		},
	}

	pretty := tree.Pretty()
	if pretty.Content == "" {
		t.Error("Pretty() should return non-empty content")
	}

	children := tree.GetChildren()
	if len(children) != 1 {
		t.Errorf("Expected 1 child, got %d", len(children))
	}
}

func TestDataDirFileInfoTreeNode(t *testing.T) {
	file := &DataDirFileInfo{
		Name:     "postgresql.conf",
		Path:     "/test/postgresql.conf",
		Size:     2048,
		MD5:      "def67890",
		Modified: time.Now().Add(-2 * time.Hour),
	}

	pretty := file.Pretty()
	if pretty.Content == "" {
		t.Error("Pretty() should return non-empty content")
	}

	if file.GetChildren() != nil {
		t.Error("File should have no children")
	}
}

func TestFormatRelativeTime(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"just now", 30 * time.Second, "just now"},
		{"1 min ago", 1 * time.Minute, "1 min ago"},
		{"5 mins ago", 5 * time.Minute, "5 mins ago"},
		{"1 hour ago", 1 * time.Hour, "1 hour ago"},
		{"2 hours ago", 2 * time.Hour, "2 hours ago"},
		{"yesterday", 24 * time.Hour, "yesterday"},
		{"2 days ago", 48 * time.Hour, "2 days ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testTime := time.Now().Add(-tt.duration)
			result := formatRelativeTime(testTime)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}
