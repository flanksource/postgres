package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/flanksource/postgres/pkg/config"
	"github.com/flanksource/postgres/pkg/sysinfo"
)

type WalInfo struct {
	LSN           string `json:"lsn"`
	LastFile      string `json:"last_file"`
	Size          int64  `json:"size" pretty:"format=bytes"`
	ArchivedCount int    `json:"archived_count"`
	ArchivedSize  int64  `json:"archived_size" pretty:"format=bytes"`
}

type Checkpoint struct {
	Last         time.Time `json:"last"`
	RedoLSN      string    `json:"redo_lsn"`
	BytesWritten int64     `json:"bytes_written" pretty:"bytes"`
}

type PostgresInfo struct {
	// e.g. 16 or 17
	VersionNumber int                `json:"version"`
	VersionInfo   string             `json:"version_info"`
	Running       bool               `json:"running"`
	DataDirectory string             `json:"data_dir"`
	BinDir        string             `json:"bin_dir"`
	ListenAddress string             `json:"listen_address"`
	Port          int                `json:"port"`
	System        sysinfo.SystemInfo `json:"system,omitempty"`

	// Configuration from disk (postgresql.conf + postgres.auto.conf)
	Conf config.Conf `json:"conf,omitempty"`

	// Runtime configuration from running instance (only populated if running)
	RuntimeConf config.Conf `json:"runtime_conf,omitempty"`

	// Size of folders on disk
	DataSize int64 `json:"data_size" pretty:"format=bytes"`
	// Size of all DBs combined
	DBSize int64 `json:"db_size" pretty:"format=bytes"`

	// pg_controldata
	ClusterState     string     `json:"cluster_state"`
	SystemIdentifier string     `json:"system_identifier"`
	FullVersion      string     `json:"full_version"`
	WalInfo          WalInfo    `json:"wal_info,omitempty"`
	Checkpoint       Checkpoint `json:"checkpoint,omitempty"`
}

func calculateDirectorySize(dirPath string) (int64, error) {
	var totalSize int64

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil && os.IsNotExist(err) {
		return 0, nil
	}

	return totalSize, err
}

func (p *Postgres) Info() (*PostgresInfo, error) {
	if err := p.ensureBinDir(); err != nil {
		return nil, fmt.Errorf("failed to resolve binary directory: %w", err)
	}

	info := &PostgresInfo{
		DataDirectory: p.DataDir,
		BinDir:        p.BinDir,
	}

	version, err := p.DetectVersion()
	if err == nil {
		info.VersionNumber = version
		pgVersion := p.GetVersion()
		info.VersionInfo = string(pgVersion)
	}

	info.Running = p.IsRunning()

	diskConf := p.GetConf()
	info.Conf = diskConf

	if portStr, ok := diskConf["port"]; ok {
		if port, err := strconv.Atoi(portStr); err == nil {
			info.Port = port
		}
	}
	if info.Port == 0 {
		info.Port = 5432
	}

	if listenAddr, ok := diskConf["listen_addresses"]; ok {
		info.ListenAddress = listenAddr
	}
	if info.ListenAddress == "" {
		info.ListenAddress = "localhost"
	}

	if info.Running {
		if runtimeConf, err := p.GetCurrentConf(); err == nil {
			info.RuntimeConf = runtimeConf.ToMap()
		}
	}

	if walSize, err := calculateDirectorySize(filepath.Join(p.DataDir, "pg_wal")); err == nil {
		info.WalInfo.Size = walSize
	}

	if dataSize, err := calculateDirectorySize(p.DataDir); err == nil {
		info.DataSize = dataSize
	}

	if info.Running {
		if results, err := p.SQL("SELECT SUM(pg_database_size(datname))::bigint FROM pg_database"); err == nil && len(results) > 0 {
			if sumVal, ok := results[0]["sum"]; ok && sumVal != nil {
				switch v := sumVal.(type) {
				case int64:
					info.DBSize = v
				case int:
					info.DBSize = int64(v)
				case float64:
					info.DBSize = int64(v)
				}
			}
		}
	}

	if controlData, err := p.GetControlData(); err == nil {
		info.ClusterState = controlData.DatabaseClusterState
		info.SystemIdentifier = controlData.DatabaseSystemIdentifier

		info.WalInfo.LSN = controlData.LatestCheckpointLocation

		info.Checkpoint = Checkpoint{
			Last:    controlData.LatestCheckpointTime,
			RedoLSN: controlData.LatestCheckpointREDOLocation,
		}
	}

	if sysInfo, err := sysinfo.DetectSystemInfo(); err == nil {
		info.System = *sysInfo
	}

	return info, nil
}
