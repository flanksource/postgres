package pgtune

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/api"
	"github.com/flanksource/postgres/pkg"
	"github.com/flanksource/postgres/pkg/config"
	"github.com/flanksource/postgres/pkg/sysinfo"
	. "github.com/flanksource/postgres/pkg/types"
	"github.com/samber/lo"
)

// TuningConfig contains the input parameters for PostgreSQL tuning
type TuningConfig struct {
	sysinfo.Resources

	PostgreSQLVersion float64

	// MaxConnections is the maximum number of database connections
	MaxConnections int

	// DBType is the database workload type
	DBType string

	OSType sysinfo.OSType

	// DiskType overrides detected disk type if specified
	DiskType *sysinfo.DiskType
}

// TunedParameters contains the calculated PostgreSQL parameters
type TunedParameters struct {
	// Memory parameters (in KB unless noted)
	SharedBuffers      Size `pretty:"format=bytes" json:"shared_buffers,omitempty"`       // shared_buffers
	EffectiveCacheSize Size `pretty:"format=bytes" json:"effective_cache_size,omitempty"` // effective_cache_size
	MaintenanceWorkMem Size `pretty:"format=bytes" json:"maintenance_work_mem,omitempty"` // maintenance_work_mem
	WorkMem            Size `pretty:"format=bytes" json:"work_mem,omitempty"`             // work_mem

	// WAL parameters (in KB unless noted)
	WalBuffers                 Size    `pretty:"format=bytes" json:"wal_buffers,omitempty"`  // wal_buffers
	MinWalSize                 Size    `pretty:"format=bytes" json:"min_wal_size,omitempty"` // min_wal_size
	MaxWalSize                 Size    `pretty:"format=bytes" json:"max_wal_size,omitempty"` // max_wal_size
	CheckpointCompletionTarget float64 `json:"checkpoint_completion_target,omitempty"`       // checkpoint_completion_target

	// Performance parameters
	RandomPageCost          float64 `json:"random_page_cost,omitempty"`          // random_page_cost
	EffectiveIoConcurrency  *int    `json:"effective_io_concurrency,omitempty"`  // effective_io_concurrency (nil if not applicable)
	DefaultStatisticsTarget int     `json:"default_statistics_target,omitempty"` // default_statistics_target

	// Parallel processing parameters
	MaxWorkerProcesses            int  `json:"max_worker_processes,omitempty"`             // max_worker_processes
	MaxParallelWorkers            int  `json:"max_parallel_workers,omitempty"`             // max_parallel_workers (PG 10+)
	MaxParallelWorkersPerGather   int  `json:"max_parallel_workers_per_gather,omitempty"`  // max_parallel_workers_per_gather
	MaxParallelMaintenanceWorkers *int `json:"max_parallel_maintenance_workers,omitempty"` // max_parallel_maintenance_workers (PG 11+)

	// WAL level settings
	WalLevel      string `json:"wal_level,omitempty"`       // wal_level
	MaxWalSenders *int   `json:"max_wal_senders,omitempty"` // max_wal_senders (set to 0 when wal_level=minimal)

	// Connection parameters
	MaxConnections int `json:"max_connections,omitempty"` // max_connections

	// Huge pages setting
	HugePages string `json:"huge_pages,omitempty"` // huge_pages

	// Warning messages for memory constraints
	Warnings []string `json:"warnings,omitempty"`
}

func (tp TunedParameters) AsConf() (*config.Conf, error) {

	data, err := json.Marshal(tp)
	if err != nil {
		return nil, err
	}
	m := map[string]any{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	conf := config.Conf{}
	for k, v := range m {
		switch val := v.(type) {
		case float64:
			if val == math.Trunc(val) {
				conf[k] = fmt.Sprintf("%.0f", val)
			} else {
				conf[k] = fmt.Sprintf("%.2f", val)
			}
		default:
			conf[k] = fmt.Sprintf("%v", val)
		}
	}

	return &conf, nil
}

func (tp TunedParameters) Pretty() api.Text {
	result := clicky.Text("Tuned PostgreSQL Parameters:", "bold").NewLine()
	result = result.Append(clicky.MustFormat(tp, clicky.FormatOptions{Format: "pretty"}))
	return result
}

// Constants for size calculations (in bytes)

// CalculateOptimalConfig generates optimal PostgreSQL configuration based on system info
func CalculateOptimalConfig(config *TuningConfig) (*TunedParameters, error) {

	params := &TunedParameters{
		MaxConnections: config.MaxConnections,
	}

	// Add warnings for extreme memory situations
	params.addMemoryWarnings(config.Mem())

	// Calculate shared_buffers
	params.SharedBuffers = config.Mem() / 4
	// Calculate effective_cache_size
	params.EffectiveCacheSize = calculateEffectiveCacheSize(*config)

	// Calculate maintenance_work_mem
	params.MaintenanceWorkMem = calculateMaintenanceWorkMem(*config)

	// Calculate WAL buffers
	params.WalBuffers = calculateWalBuffers(params.SharedBuffers)

	// Calculate WAL sizes
	params.MinWalSize, params.MaxWalSize = calculateWalSizes(config.DBType)

	// Set checkpoint completion target
	params.CheckpointCompletionTarget = 0.9

	// Calculate disk-related parameters
	params.RandomPageCost = 4.0
	params.EffectiveIoConcurrency = lo.ToPtr(200)

	// Calculate default statistics target
	params.DefaultStatisticsTarget = calculateDefaultStatisticsTarget(config.DBType)

	params.MaxWorkerProcesses, params.MaxParallelWorkersPerGather,
		params.MaxParallelWorkers, params.MaxParallelMaintenanceWorkers =
		calculateParallelSettings(config.CPUs, config.DBType, config.PostgreSQLVersion)

	// Calculate work_mem (depends on parallel settings)
	params.WorkMem = calculateWorkMem(config.Mem(), params.SharedBuffers, config.MaxConnections,
		params.MaxWorkerProcesses, config.DBType)

	// Set WAL level
	params.WalLevel, params.MaxWalSenders = calculateWalLevel(config.DBType)

	// Set huge pages
	params.HugePages = calculateHugePages(config.Mem())

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

// calculateEffectiveCacheSize calculates optimal effective_cache_size
func calculateEffectiveCacheSize(t TuningConfig) Size {
	switch t.DBType {
	case sysinfo.DBTypeDesktop:
		return t.Resources.Mem() / 4 // 1/4 for desktop
	default:
		return (t.Resources.Mem() * 3) / 4 // 3/4 for web, oltp, dw, mixed
	}
}

// calculateMaintenanceWorkMem calculates optimal maintenance_work_mem
func calculateMaintenanceWorkMem(t TuningConfig) Size {
	var maintenanceMem Size

	switch t.DBType {
	case sysinfo.DBTypeDW:
		maintenanceMem = t.Resources.Mem() / 8 // 1/8 for data warehouse
	default:
		maintenanceMem = t.Resources.Mem() / 16 // 1/16 for others
	}

	// Cap at 2GB
	maxLimit := 2 * GB
	if maintenanceMem >= maxLimit {
		if t.OSType == sysinfo.OSWindows {
			// Windows: 2GB - 1MB to avoid errors
			maintenanceMem = maxLimit - MB
		} else {
			maintenanceMem = maxLimit
		}
	}

	return maintenanceMem
}

// calculateWorkMem calculates optimal work_mem
func calculateWorkMem(totalMemoryKB, sharedBuffers Size, maxConnections, maxWorkerProcesses int, dbType string) Size {
	// Formula: (total_memory - shared_buffers) / ((max_connections + max_worker_processes) * 3)
	availableMemory := totalMemoryKB - sharedBuffers
	totalProcesses := maxConnections + maxWorkerProcesses

	baseWorkMem := availableMemory / Size(totalProcesses*3)

	var workMem Size
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
	if workMem < 512*KB {
		workMem = 512 * KB
	}

	return workMem
}

// calculateWalBuffers calculates optimal wal_buffers
func calculateWalBuffers(sharedBuffers Size) Size {
	// 3% of shared_buffers, max 16MB
	walBuffers := (3 * sharedBuffers) / 100
	maxWalBuffer := 16 * MB

	if walBuffers > maxWalBuffer {
		walBuffers = maxWalBuffer
	}

	// Round up to 16MB if close (for Windows with 512MB shared_buffers)
	nearValue := 14 * MB
	if walBuffers > nearValue && walBuffers < maxWalBuffer {
		walBuffers = maxWalBuffer
	}

	// Minimum is 32KB
	if walBuffers < MB {
		walBuffers = MB
	}

	return walBuffers
}

// calculateWalSizes calculates min_wal_size and max_wal_size
func calculateWalSizes(dbType string) (Size, Size) {
	var minWal, maxWal Size

	switch dbType {
	case sysinfo.DBTypeWeb:
		minWal = GB
		maxWal = 4 * GB
	case sysinfo.DBTypeOLTP:
		minWal = 2 * GB
		maxWal = 8 * GB
	case sysinfo.DBTypeDW:
		minWal = 1 * GB
		maxWal = 16 * GB
	case sysinfo.DBTypeDesktop:
		minWal = 64 * MB
		maxWal = 2 * GB
	case sysinfo.DBTypeMixed:
		minWal = 1 * GB
		maxWal = 4 * GB
	}

	return minWal, maxWal
}

// calculateDefaultStatisticsTarget calculates default_statistics_target
func calculateDefaultStatisticsTarget(dbType string) int {
	switch dbType {
	case sysinfo.DBTypeDW:
		return 500
	default:
		return 100
	}
}

// calculateParallelSettings calculates parallel processing parameters
func calculateParallelSettings(cpuCount int, dbType string, pgVersion float64) (int, int, int, *int) {
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
	maxParallelWorkers = cpuCount
	parallelMaintenance := int(math.Ceil(float64(cpuCount) / 2))
	if parallelMaintenance > 4 {
		parallelMaintenance = 4
	}
	maxParallelMaintenanceWorkers = &parallelMaintenance

	return maxWorkerProcesses, workersPerGather, maxParallelWorkers, maxParallelMaintenanceWorkers
}

// calculateWalLevel calculates WAL level settings
func calculateWalLevel(dbType string) (string, *int) {
	if dbType == sysinfo.DBTypeDesktop {
		// Desktop workload uses minimal WAL
		maxWalSenders := 0
		return "minimal", &maxWalSenders
	}

	// Default to replica for other workloads
	return "replica", nil
}

// calculateHugePages calculates huge_pages setting
func calculateHugePages(totalMemoryKB Size) string {
	// Enable huge pages for systems with 32GB+ memory
	if totalMemoryKB > 32*GB {
		return "try"
	}
	return "off"
}

// addMemoryWarnings adds warnings for extreme memory situations
func (tp *TunedParameters) addMemoryWarnings(totalMemoryBytes Size) {
	if totalMemoryBytes < 256*MB {
		tp.Warnings = append(tp.Warnings,
			"WARNING: This tool is not optimal for low memory systems (< 256MB)")
	}

	if totalMemoryBytes > 100*GB {
		tp.Warnings = append(tp.Warnings,
			"WARNING: This tool is not optimal for very high memory systems (> 100GB)")
	}
}

// GetRecommendedMaxConnections returns recommended max_connections based on DB type
func GetRecommendedMaxConnections(dbType string) int {
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
