package config

import (
	"strconv"
	"strings"
	"time"
)

type ControlData struct {
	PgControlVersion             int
	CatalogVersion               int
	DatabaseSystemIdentifier     string
	DatabaseClusterState         string
	PgControlLastModified        time.Time
	LatestCheckpointLocation     string
	LatestCheckpointREDOLocation string
	LatestCheckpointREDOWALFile  string
	LatestCheckpointTimeLineID   int
	LatestCheckpointTime         time.Time
	WalLevel                     string
	WalLogHints                  string
	MaxConnections               int
	MaxWorkerProcesses           int
	MaxWalSenders                int
	MaxPreparedXacts             int
	MaxLocksPerXact              int
	TrackCommitTimestamp         string
	DatabaseBlockSize            int
	WALBlockSize                 int
	BytesPerWALSegment           int
	MaxIdentifierLength          int
	DataPageChecksumVersion      int
}

func ParseControlData(output string) (*ControlData, error) {
	cd := &ControlData{}
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "pg_control version number":
			if v, err := strconv.Atoi(value); err == nil {
				cd.PgControlVersion = v
			}
		case "Catalog version number":
			if v, err := strconv.Atoi(value); err == nil {
				cd.CatalogVersion = v
			}
		case "Database system identifier":
			cd.DatabaseSystemIdentifier = value
		case "Database cluster state":
			cd.DatabaseClusterState = value
		case "pg_control last modified":
			if t, err := time.Parse("Mon Jan 2 15:04:05 2006", value); err == nil {
				cd.PgControlLastModified = t
			}
		case "Latest checkpoint location":
			cd.LatestCheckpointLocation = value
		case "Latest checkpoint's REDO location":
			cd.LatestCheckpointREDOLocation = value
		case "Latest checkpoint's REDO WAL file":
			cd.LatestCheckpointREDOWALFile = value
		case "Latest checkpoint's TimeLineID":
			if v, err := strconv.Atoi(value); err == nil {
				cd.LatestCheckpointTimeLineID = v
			}
		case "Time of latest checkpoint":
			if t, err := time.Parse("Mon Jan 2 15:04:05 2006", value); err == nil {
				cd.LatestCheckpointTime = t
			}
		case "wal_level setting":
			cd.WalLevel = value
		case "wal_log_hints setting":
			cd.WalLogHints = value
		case "max_connections setting":
			if v, err := strconv.Atoi(value); err == nil {
				cd.MaxConnections = v
			}
		case "max_worker_processes setting":
			if v, err := strconv.Atoi(value); err == nil {
				cd.MaxWorkerProcesses = v
			}
		case "max_wal_senders setting":
			if v, err := strconv.Atoi(value); err == nil {
				cd.MaxWalSenders = v
			}
		case "max_prepared_xacts setting":
			if v, err := strconv.Atoi(value); err == nil {
				cd.MaxPreparedXacts = v
			}
		case "max_locks_per_xact setting":
			if v, err := strconv.Atoi(value); err == nil {
				cd.MaxLocksPerXact = v
			}
		case "track_commit_timestamp setting":
			cd.TrackCommitTimestamp = value
		case "Database block size":
			if v, err := strconv.Atoi(value); err == nil {
				cd.DatabaseBlockSize = v
			}
		case "WAL block size":
			if v, err := strconv.Atoi(value); err == nil {
				cd.WALBlockSize = v
			}
		case "Bytes per WAL segment":
			if v, err := strconv.Atoi(value); err == nil {
				cd.BytesPerWALSegment = v
			}
		case "Maximum length of identifiers":
			if v, err := strconv.Atoi(value); err == nil {
				cd.MaxIdentifierLength = v
			}
		case "Data page checksum version":
			if v, err := strconv.Atoi(value); err == nil {
				cd.DataPageChecksumVersion = v
			}
		}
	}

	return cd, nil
}
