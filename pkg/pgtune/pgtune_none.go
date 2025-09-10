//go:build pgtune_none

package pgtune

import (
	"fmt"

	"github.com/flanksource/postgres/pkg"
	"github.com/flanksource/postgres/pkg/sysinfo"
)

// TuningConfig contains the input parameters for PostgreSQL tuning
type TuningConfig struct {
	SystemInfo     *sysinfo.SystemInfo
	MaxConnections int
	DBType         sysinfo.DBType
	DiskType       *sysinfo.DiskType
}

// TunedParameters contains the calculated PostgreSQL parameters
type TunedParameters struct {
	SharedBuffers                 uint64
	EffectiveCacheSize           uint64
	MaintenanceWorkMem           uint64
	WorkMem                      uint64
	WalBuffers                   uint64
	MinWalSize                   uint64
	MaxWalSize                   uint64
	CheckpointCompletionTarget   float64
	RandomPageCost               float64
	EffectiveIoConcurrency       *int
	DefaultStatisticsTarget      int
	MaxWorkerProcesses           int
	MaxParallelWorkers           int
	MaxParallelWorkersPerGather  int
	MaxParallelMaintenanceWorkers *int
	WalLevel                     string
	MaxWalSenders                *int
	MaxConnections               int
	HugePages                    string
	Warnings                     []string
}

// CalculateOptimalConfig is a stub that returns an error when pgtune is disabled
func CalculateOptimalConfig(config *TuningConfig) (*TunedParameters, error) {
	return nil, fmt.Errorf("pgtune is disabled in this build (built with pgtune_none tag)")
}

// PostProcessor is a function type for processing PostgreSQL configuration
type PostProcessor func(*pkg.PostgresConf, *sysinfo.SystemInfo) error

var postProcessors []PostProcessor

// RegisterPostProcessor is a stub that does nothing when pgtune is disabled
func RegisterPostProcessor(processor PostProcessor) {
	// No-op when pgtune is disabled
}

// ApplyPostProcessors is a stub that does nothing when pgtune is disabled
func ApplyPostProcessors(pgConf *pkg.PostgresConf, sysInfo *sysinfo.SystemInfo) error {
	// No-op when pgtune is disabled
	return nil
}

// GetRecommendedMaxConnections returns a default value when pgtune is disabled
func GetRecommendedMaxConnections(dbType sysinfo.DBType) int {
	return 100 // Default safe value
}

// FormatAsKB is a stub method for TunedParameters
func (tp *TunedParameters) FormatAsKB(valueKB uint64) string {
	return fmt.Sprintf("%dkB", valueKB)
}