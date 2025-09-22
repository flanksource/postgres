//go:build !pgtune_none

package pgtune

import (
	"fmt"
	"math"

	"github.com/flanksource/postgres/pkg"
	"github.com/flanksource/postgres/pkg/sysinfo"
	"github.com/flanksource/postgres/pkg/types"
	"github.com/flanksource/postgres/pkg/utils"
)

// TuningConfig contains the input parameters for PostgreSQL tuning
type TuningConfig struct {
	// SystemInfo contains detected system information
	SystemInfo *sysinfo.SystemInfo

	// MaxConnections is the maximum number of database connections
	MaxConnections int

	// DBType is the database workload type
	DBType sysinfo.DBType

	// DiskType overrides detected disk type if specified
	DiskType *sysinfo.DiskType
}

// TunedParameters contains the calculated PostgreSQL parameters
type TunedParameters struct {
	// Memory parameters (in KB unless noted)
	SharedBuffers      uint64 // shared_buffers
	EffectiveCacheSize uint64 // effective_cache_size
	MaintenanceWorkMem uint64 // maintenance_work_mem
	WorkMem            uint64 // work_mem

	// WAL parameters (in KB unless noted)
	WalBuffers                 uint64  // wal_buffers
	MinWalSize                 uint64  // min_wal_size
	MaxWalSize                 uint64  // max_wal_size
	CheckpointCompletionTarget float64 // checkpoint_completion_target

	// Performance parameters
	RandomPageCost          float64 // random_page_cost
	EffectiveIoConcurrency  *int    // effective_io_concurrency (nil if not applicable)
	DefaultStatisticsTarget int     // default_statistics_target

	// Parallel processing parameters
	MaxWorkerProcesses            int  // max_worker_processes
	MaxParallelWorkers            int  // max_parallel_workers (PG 10+)
	MaxParallelWorkersPerGather   int  // max_parallel_workers_per_gather
	MaxParallelMaintenanceWorkers *int // max_parallel_maintenance_workers (PG 11+)

	// WAL level settings
	WalLevel      string // wal_level
	MaxWalSenders *int   // max_wal_senders (set to 0 when wal_level=minimal)

	// Connection parameters
	MaxConnections int // max_connections

	// Huge pages setting
	HugePages string // huge_pages

	// Warning messages for memory constraints
	Warnings []string
}

// Constants for size calculations (in bytes)
const (
	KB = 1024
	MB = 1024 * KB
	GB = 1024 * MB
	TB = 1024 * GB
)

// CalculateOptimalConfig generates optimal PostgreSQL configuration based on system info
func CalculateOptimalConfig(config *TuningConfig) (*TunedParameters, error) {
	if config.SystemInfo == nil {
		return nil, fmt.Errorf("system info is required")
	}

	if config.MaxConnections <= 0 {
		return nil, fmt.Errorf("max_connections must be positive")
	}

	sysInfo := config.SystemInfo
	totalMemoryKB := sysInfo.TotalMemoryKB()

	// Use override disk type if provided
	diskType := sysInfo.DiskType
	if config.DiskType != nil {
		diskType = *config.DiskType
	}

	params := &TunedParameters{
		MaxConnections: config.MaxConnections,
	}

	// Add warnings for extreme memory situations
	params.addMemoryWarnings(sysInfo.TotalMemoryBytes)

	// Calculate shared_buffers
	params.SharedBuffers = calculateSharedBuffers(totalMemoryKB, config.DBType, sysInfo.OSType, sysInfo.PostgreSQLVersion)

	// Calculate effective_cache_size
	params.EffectiveCacheSize = calculateEffectiveCacheSize(totalMemoryKB, config.DBType)

	// Calculate maintenance_work_mem
	params.MaintenanceWorkMem = calculateMaintenanceWorkMem(totalMemoryKB, config.DBType, sysInfo.OSType)

	// Calculate WAL buffers
	params.WalBuffers = calculateWalBuffers(params.SharedBuffers)

	// Calculate WAL sizes
	params.MinWalSize, params.MaxWalSize = calculateWalSizes(config.DBType)

	// Set checkpoint completion target
	params.CheckpointCompletionTarget = 0.9

	// Calculate disk-related parameters
	params.RandomPageCost = calculateRandomPageCost(diskType)
	params.EffectiveIoConcurrency = calculateEffectiveIoConcurrency(sysInfo.OSType, diskType)

	// Calculate default statistics target
	params.DefaultStatisticsTarget = calculateDefaultStatisticsTarget(config.DBType)

	// Calculate parallel processing parameters
	params.MaxWorkerProcesses, params.MaxParallelWorkersPerGather,
		params.MaxParallelWorkers, params.MaxParallelMaintenanceWorkers =
		calculateParallelSettings(sysInfo.CPUCount, config.DBType, sysInfo.PostgreSQLVersion)

	// Calculate work_mem (depends on parallel settings)
	params.WorkMem = calculateWorkMem(totalMemoryKB, params.SharedBuffers, config.MaxConnections,
		params.MaxWorkerProcesses, config.DBType)

	// Set WAL level
	params.WalLevel, params.MaxWalSenders = calculateWalLevel(config.DBType)

	// Set huge pages
	params.HugePages = calculateHugePages(totalMemoryKB)

	return params, nil
}

// PostProcessor is a function type for processing PostgreSQL configuration
type PostProcessor func(*pkg.PostgresConf, *sysinfo.SystemInfo) error

var postProcessors []PostProcessor

// RegisterPostProcessor registers a new post-processor function
func RegisterPostProcessor(processor PostProcessor) {
	postProcessors = append(postProcessors, processor)
}

// ApplyPostProcessors applies all registered post-processors to the configuration
func ApplyPostProcessors(pgConf *pkg.PostgresConf, sysInfo *sysinfo.SystemInfo) error {
	for _, processor := range postProcessors {
		if err := processor(pgConf, sysInfo); err != nil {
			return err
		}
	}
	return nil
}

// tuningPostProcessor is the default tuning post-processor
func tuningPostProcessor(pgConf *pkg.PostgresConf, sysInfo *sysinfo.SystemInfo) error {
	// Create tuning config from the PostgresConf and system info
	maxConn := 100 // default value
	if pgConf.MaxConnections != 0 {
		maxConn = pgConf.MaxConnections
	}
	config := &TuningConfig{
		SystemInfo:     sysInfo,
		MaxConnections: maxConn,
		DBType:         sysinfo.DBTypeMixed, // Default to mixed workload
		DiskType:       nil,                 // Use detected disk type
	}

	// Calculate optimal parameters
	params, err := CalculateOptimalConfig(config)
	if err != nil {
		return fmt.Errorf("failed to calculate optimal config: %w", err)
	}

	// Update the PostgresConf model with the calculated values
	pgConf.MaxConnections = params.MaxConnections
	sharedBuffersSize := types.Size(utils.KBToBytes(params.SharedBuffers))
	pgConf.SharedBuffers = sharedBuffersSize.String()

	// Only set fields that exist in the new PostgresConf struct
	// The rest will be handled by the generators when creating config files

	return nil
}

// init registers the default tuning post-processor
func init() {
	RegisterPostProcessor(tuningPostProcessor)
}

// calculateSharedBuffers calculates optimal shared_buffers value
func calculateSharedBuffers(totalMemoryKB uint64, dbType sysinfo.DBType, osType sysinfo.OSType, pgVersion float64) uint64 {
	return totalMemoryKB / 4
}

// calculateEffectiveCacheSize calculates optimal effective_cache_size
func calculateEffectiveCacheSize(totalMemoryKB uint64, dbType sysinfo.DBType) uint64 {
	switch dbType {
	case sysinfo.DBTypeDesktop:
		return totalMemoryKB / 4 // 1/4 for desktop
	default:
		return (totalMemoryKB * 3) / 4 // 3/4 for web, oltp, dw, mixed
	}
}

// calculateMaintenanceWorkMem calculates optimal maintenance_work_mem
func calculateMaintenanceWorkMem(totalMemoryKB uint64, dbType sysinfo.DBType, osType sysinfo.OSType) uint64 {
	var maintenanceMem uint64

	switch dbType {
	case sysinfo.DBTypeDW:
		maintenanceMem = totalMemoryKB / 8 // 1/8 for data warehouse
	default:
		maintenanceMem = totalMemoryKB / 16 // 1/16 for others
	}

	// Cap at 2GB
	maxLimit := uint64(2 * GB / KB)
	if maintenanceMem >= maxLimit {
		if osType == sysinfo.OSWindows {
			// Windows: 2GB - 1MB to avoid errors
			maintenanceMem = maxLimit - uint64(1*MB/KB)
		} else {
			maintenanceMem = maxLimit
		}
	}

	return maintenanceMem
}

// calculateWorkMem calculates optimal work_mem
func calculateWorkMem(totalMemoryKB, sharedBuffers uint64, maxConnections, maxWorkerProcesses int, dbType sysinfo.DBType) uint64 {
	// Formula: (total_memory - shared_buffers) / ((max_connections + max_worker_processes) * 3)
	availableMemory := totalMemoryKB - sharedBuffers
	totalProcesses := uint64(maxConnections + maxWorkerProcesses)

	baseWorkMem := availableMemory / (totalProcesses * 3)

	var workMem uint64
	switch dbType {
	case sysinfo.DBTypeDW:
		workMem = baseWorkMem / 2
	case sysinfo.DBTypeDesktop:
		workMem = baseWorkMem / 6
	case sysinfo.DBTypeMixed:
		workMem = baseWorkMem / 2
	default: // web, oltp
		workMem = baseWorkMem
	}

	// Minimum work_mem is 512KB
	if workMem < 512 {
		workMem = 512
	}

	return workMem
}

// calculateWalBuffers calculates optimal wal_buffers
func calculateWalBuffers(sharedBuffers uint64) uint64 {
	// 3% of shared_buffers, max 16MB
	walBuffers := (3 * sharedBuffers) / 100
	maxWalBuffer := uint64(16 * MB / KB)

	if walBuffers > maxWalBuffer {
		walBuffers = maxWalBuffer
	}

	// Round up to 16MB if close (for Windows with 512MB shared_buffers)
	nearValue := uint64(14 * MB / KB)
	if walBuffers > nearValue && walBuffers < maxWalBuffer {
		walBuffers = maxWalBuffer
	}

	// Minimum is 32KB
	if walBuffers < 32 {
		walBuffers = 32
	}

	return walBuffers
}

// calculateWalSizes calculates min_wal_size and max_wal_size
func calculateWalSizes(dbType sysinfo.DBType) (uint64, uint64) {
	var minWal, maxWal uint64

	switch dbType {
	case sysinfo.DBTypeWeb:
		minWal = uint64(1024 * MB / KB) // 1GB
		maxWal = uint64(4096 * MB / KB) // 4GB
	case sysinfo.DBTypeOLTP:
		minWal = uint64(2048 * MB / KB) // 2GB
		maxWal = uint64(8192 * MB / KB) // 8GB
	case sysinfo.DBTypeDW:
		minWal = uint64(4096 * MB / KB)  // 4GB
		maxWal = uint64(16384 * MB / KB) // 16GB
	case sysinfo.DBTypeDesktop:
		minWal = uint64(100 * MB / KB)  // 100MB
		maxWal = uint64(2048 * MB / KB) // 2GB
	case sysinfo.DBTypeMixed:
		minWal = uint64(1024 * MB / KB) // 1GB
		maxWal = uint64(4096 * MB / KB) // 4GB
	}

	return minWal, maxWal
}

// calculateRandomPageCost calculates random_page_cost based on disk type
func calculateRandomPageCost(diskType sysinfo.DiskType) float64 {
	switch diskType {
	case sysinfo.DiskHDD:
		return 4.0
	case sysinfo.DiskSSD, sysinfo.DiskSAN:
		return 1.1
	default:
		return 1.1 // Default to SSD
	}
}

// calculateEffectiveIoConcurrency calculates effective_io_concurrency
func calculateEffectiveIoConcurrency(osType sysinfo.OSType, diskType sysinfo.DiskType) *int {
	// Only available on Linux
	if osType != sysinfo.OSLinux {
		return nil
	}

	var concurrency int
	switch diskType {
	case sysinfo.DiskHDD:
		concurrency = 2
	case sysinfo.DiskSSD:
		concurrency = 200
	case sysinfo.DiskSAN:
		concurrency = 300
	default:
		concurrency = 200 // Default to SSD
	}

	return &concurrency
}

// calculateDefaultStatisticsTarget calculates default_statistics_target
func calculateDefaultStatisticsTarget(dbType sysinfo.DBType) int {
	switch dbType {
	case sysinfo.DBTypeDW:
		return 500
	default:
		return 100
	}
}

// calculateParallelSettings calculates parallel processing parameters
func calculateParallelSettings(cpuCount int, dbType sysinfo.DBType, pgVersion float64) (int, int, int, *int) {
	if cpuCount < 4 {
		// Default values for systems with < 4 CPUs
		return 8, 2, 8, nil
	}

	maxWorkerProcesses := cpuCount

	// Calculate workers per gather
	workersPerGather := int(math.Ceil(float64(cpuCount) / 2))
	if dbType != sysinfo.DBTypeDW && workersPerGather > 4 {
		workersPerGather = 4 // Limit for non-DW workloads
	}

	var maxParallelWorkers int
	var maxParallelMaintenanceWorkers *int

	// max_parallel_workers available in PG 10+
	if pgVersion >= 10 {
		maxParallelWorkers = cpuCount
	} else {
		maxParallelWorkers = 8 // Default for older versions
	}

	// max_parallel_maintenance_workers available in PG 11+
	if pgVersion >= 11 {
		parallelMaintenance := int(math.Ceil(float64(cpuCount) / 2))
		if parallelMaintenance > 4 {
			parallelMaintenance = 4
		}
		maxParallelMaintenanceWorkers = &parallelMaintenance
	}

	return maxWorkerProcesses, workersPerGather, maxParallelWorkers, maxParallelMaintenanceWorkers
}

// calculateWalLevel calculates WAL level settings
func calculateWalLevel(dbType sysinfo.DBType) (string, *int) {
	if dbType == sysinfo.DBTypeDesktop {
		// Desktop workload uses minimal WAL
		maxWalSenders := 0
		return "minimal", &maxWalSenders
	}

	// Default to replica for other workloads
	return "replica", nil
}

// calculateHugePages calculates huge_pages setting
func calculateHugePages(totalMemoryKB uint64) string {
	// Enable huge pages for systems with 32GB+ memory
	if totalMemoryKB >= uint64(32*GB/KB) {
		return "try"
	}
	return "off"
}

// addMemoryWarnings adds warnings for extreme memory situations
func (tp *TunedParameters) addMemoryWarnings(totalMemoryBytes uint64) {
	if totalMemoryBytes < 256*MB {
		tp.Warnings = append(tp.Warnings,
			"WARNING: This tool is not optimal for low memory systems (< 256MB)")
	}

	if totalMemoryBytes > 100*GB {
		tp.Warnings = append(tp.Warnings,
			"WARNING: This tool is not optimal for very high memory systems (> 100GB)")
	}
}

// FormatAsKB formats a value in KB as a PostgreSQL configuration value
func (tp *TunedParameters) FormatAsKB(valueKB uint64) string {
	if valueKB >= GB/KB {
		return fmt.Sprintf("%dGB", valueKB/(GB/KB))
	}
	if valueKB >= MB/KB {
		return fmt.Sprintf("%dMB", valueKB/(MB/KB))
	}
	return fmt.Sprintf("%dkB", valueKB)
}

// GetRecommendedMaxConnections returns recommended max_connections based on DB type
func GetRecommendedMaxConnections(dbType sysinfo.DBType) int {
	switch dbType {
	case sysinfo.DBTypeWeb:
		return 200
	case sysinfo.DBTypeOLTP:
		return 300
	case sysinfo.DBTypeDW:
		return 40
	case sysinfo.DBTypeDesktop:
		return 20
	case sysinfo.DBTypeMixed:
		return 100
	default:
		return 100
	}
}
