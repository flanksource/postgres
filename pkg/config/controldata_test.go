package config

import (
	"testing"
)

const sampleControlData = `pg_control version number:            1700
Catalog version number:               202406281
Database system identifier:           7545409678823081619
Database cluster state:               in production
pg_control last modified:             Wed Oct 29 08:00:54 2025
Latest checkpoint location:           0/31E725F0
Latest checkpoint's REDO location:    0/31E72598
Latest checkpoint's REDO WAL file:    000000010000000000000031
Latest checkpoint's TimeLineID:       1
Latest checkpoint's PrevTimeLineID:   1
Latest checkpoint's full_page_writes: on
Latest checkpoint's NextXID:          0:56878
Latest checkpoint's NextOID:          133774
Latest checkpoint's NextMultiXactId:  3
Latest checkpoint's NextMultiOffset:  5
Latest checkpoint's oldestXID:        730
Latest checkpoint's oldestXID's DB:   1
Latest checkpoint's oldestActiveXID:  56878
Latest checkpoint's oldestMultiXid:   1
Latest checkpoint's oldestMulti's DB: 1
Latest checkpoint's oldestCommitTsXid:0
Latest checkpoint's newestCommitTsXid:0
Time of latest checkpoint:            Wed Oct 29 08:00:53 2025
Fake LSN counter for unlogged rels:   0/3E8
Minimum recovery ending location:     0/0
Min recovery ending loc's timeline:   0
Backup start location:                0/0
Backup end location:                  0/0
End-of-backup record required:        no
wal_level setting:                    replica
wal_log_hints setting:                off
max_connections setting:              100
max_worker_processes setting:         8
max_wal_senders setting:              10
max_prepared_xacts setting:           0
max_locks_per_xact setting:           64
track_commit_timestamp setting:       off
Maximum data alignment:               8
Database block size:                  8192
Blocks per segment of large relation: 131072
WAL block size:                       8192
Bytes per WAL segment:                16777216
Maximum length of identifiers:        64
Maximum columns in an index:          32
Maximum size of a TOAST chunk:        1996
Size of a large-object chunk:         2048
Date/time type storage:               64-bit integers
Float8 argument passing:              by value
Data page checksum version:           0
Mock authentication nonce:            e22628b23a7d14804decec283d6860b4947e8d1cf211e76d609d87038f8aae0f`

func TestParseControlData(t *testing.T) {
	cd, err := ParseControlData(sampleControlData)
	if err != nil {
		t.Fatalf("ParseControlData failed: %v", err)
	}

	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"PgControlVersion", cd.PgControlVersion, 1700},
		{"CatalogVersion", cd.CatalogVersion, 202406281},
		{"DatabaseSystemIdentifier", cd.DatabaseSystemIdentifier, "7545409678823081619"},
		{"DatabaseClusterState", cd.DatabaseClusterState, "in production"},
		{"LatestCheckpointLocation", cd.LatestCheckpointLocation, "0/31E725F0"},
		{"LatestCheckpointREDOLocation", cd.LatestCheckpointREDOLocation, "0/31E72598"},
		{"LatestCheckpointREDOWALFile", cd.LatestCheckpointREDOWALFile, "000000010000000000000031"},
		{"LatestCheckpointTimeLineID", cd.LatestCheckpointTimeLineID, 1},
		{"WalLevel", cd.WalLevel, "replica"},
		{"WalLogHints", cd.WalLogHints, "off"},
		{"MaxConnections", cd.MaxConnections, 100},
		{"MaxWorkerProcesses", cd.MaxWorkerProcesses, 8},
		{"MaxWalSenders", cd.MaxWalSenders, 10},
		{"MaxPreparedXacts", cd.MaxPreparedXacts, 0},
		{"MaxLocksPerXact", cd.MaxLocksPerXact, 64},
		{"TrackCommitTimestamp", cd.TrackCommitTimestamp, "off"},
		{"DatabaseBlockSize", cd.DatabaseBlockSize, 8192},
		{"WALBlockSize", cd.WALBlockSize, 8192},
		{"BytesPerWALSegment", cd.BytesPerWALSegment, 16777216},
		{"MaxIdentifierLength", cd.MaxIdentifierLength, 64},
		{"DataPageChecksumVersion", cd.DataPageChecksumVersion, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s: got %v, expected %v", tt.name, tt.got, tt.expected)
			}
		})
	}

	if cd.PgControlLastModified.IsZero() {
		t.Error("PgControlLastModified should not be zero")
	}

	if cd.LatestCheckpointTime.IsZero() {
		t.Error("LatestCheckpointTime should not be zero")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
