// Tests starting and stopping a real local PostgreSQL cluster through pg_ctl.
// Skipped when PostgreSQL binaries are not on PATH.
package server

import (
	"os"
	osexec "os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestStartReturnsWhileServerKeepsRunning(t *testing.T) {
	pgCtl, err := osexec.LookPath("pg_ctl")
	if err != nil {
		t.Skip("pg_ctl not found on PATH")
	}
	binDir := filepath.Dir(pgCtl)
	dataDir := filepath.Join(t.TempDir(), "data")
	socketDir := t.TempDir()

	if out, err := osexec.Command(filepath.Join(binDir, "initdb"), "-D", dataDir, "-U", "postgres").CombinedOutput(); err != nil {
		t.Fatalf("initdb failed: %v\n%s", err, out)
	}
	conf, err := os.OpenFile(filepath.Join(dataDir, "postgresql.conf"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conf.WriteString("port = 55439\nlisten_addresses = ''\nunix_socket_directories = '" + socketDir + "'\n"); err != nil {
		t.Fatal(err)
	}
	conf.Close()

	p := &Postgres{DataDir: dataDir, BinDir: binDir}
	t.Cleanup(func() { _ = osexec.Command(pgCtl, "-D", dataDir, "-m", "immediate", "stop").Run() })

	started := make(chan error, 1)
	go func() { started <- p.Start() }()

	select {
	case err := <-started:
		if err != nil {
			t.Fatalf("Start failed: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Start did not return while the server kept running")
	}

	if !p.IsRunning() {
		t.Fatal("expected PostgreSQL to be running after Start")
	}
	if err := p.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}
