package sysinfo

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/api"
	"github.com/flanksource/commons/text"
	"github.com/flanksource/postgres/pkg/types"
	"github.com/shirou/gopsutil/v3/mem"
)

// OSType represents the operating system type
type OSType string

const (
	OSLinux   OSType = "linux"
	OSWindows OSType = "windows"
	OSMac     OSType = "mac"
)

// DiskType represents the type of storage device
type DiskType string

const (
	DiskSSD DiskType = "ssd"
	DiskHDD DiskType = "hdd"
	DiskSAN DiskType = "san"
)

// DBType represents the database workload type
type DBType string

const (
	DBTypeWeb     string = "web"
	DBTypeOLTP    string = "oltp"
	DBTypeDW      string = "dw"
	DBTypeDesktop string = "desktop"
	DBTypeMixed   string = "mixed"
)

// Resources represents physical system resources
type Resources struct {
	CPUs   int    `json:"cpus,omitempty"`
	Memory uint64 `json:"memory,omitempty" pretty:"format=bytes"`
	Millis int    `json:"millis,omitempty"` // CPU quota in millicores (e.g., 1500 = 1.5 CPUs)
}

func (r Resources) Mem() types.Size {
	return types.Size(r.Memory)
}

func (r Resources) Pretty() api.Text {

	t := clicky.Text("").Append("CPU: ", "text-muted").Append(r.CPUs)
	if r.Millis > 0 {
		t = t.Append(" Quota: ", "text-muted").Append(r.Millis).Append(" millis", "text-muted")
	}
	if r.Memory > 0 {
		t = t.Append(" Memory: ", "text-muted").Append(text.HumanizeBytes(r.Memory))
	}
	return t
}

func (r Resources) String() string {
	return r.Pretty().String()
}

// ContainerResources represents container resource limits

// SystemInfo contains detected system information
type SystemInfo struct {
	// Nested resource information
	System    Resources `json:"system,omitempty"`    // Physical system resources
	Container Resources `json:"container,omitempty"` // nil if not in container

	// Other system information
	IPAddresses       []string `json:"ip_addresses,omitempty"`
	OSType            OSType   `json:"os_type,omitempty"`
	PostgreSQLVersion float64  `json:"postgres_version,omitempty"`
	DiskType          DiskType `json:"disk_type,omitempty"`
	IsContainer       bool     `json:"is_container,omitempty"`
}

// DetectSystemInfo automatically detects system information
func DetectSystemInfo() (*SystemInfo, error) {
	info := &SystemInfo{}

	// Detect if running in a container
	info.IsContainer = detectContainer()

	// Detect memory - use container limits if available (defaults to 1GB if not detected)
	totalMem, containerMem, _ := detectMemoryWithContainer()
	info.System.Memory = totalMem

	// Detect CPU count
	info.System.CPUs = runtime.NumCPU()

	// Detect CPU quota from container limits
	cpuQuota := detectContainerCPUQuota()

	if containerMem > 0 {
		info.Container.Memory = containerMem
	}

	if cpuQuota > 0 {
		info.Container.Millis = int(cpuQuota * 1000.0)
		// Round up fractional CPUs for effective count
		info.Container.CPUs = int(cpuQuota + 0.5)
		if info.Container.CPUs < 1 {
			info.Container.CPUs = 1
		}
	}

	// Detect OS type
	info.OSType = detectOSType()

	info.IPAddresses = detectIPAddresses()

	// Detect PostgreSQL version from environment
	pgVersion, err := detectPostgreSQLVersion()
	if err != nil {
		// Default to PostgreSQL 17 if detection fails
		info.PostgreSQLVersion = 17.0
	} else {
		info.PostgreSQLVersion = pgVersion
	}

	// Detect disk type (default to SSD if detection fails)
	diskType, err := detectDiskType()
	if err != nil {
		info.DiskType = DiskSSD // Default assumption for modern systems
	} else {
		info.DiskType = diskType
	}

	return info, nil
}

func detectIPAddresses() []string {
	var addrs []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return addrs
	}

	for _, iface := range ifaces {
		if (iface.Flags & net.FlagUp) == 0 {
			continue // interface down
		}
		if (iface.Flags & net.FlagLoopback) != 0 {
			continue // loopback interface
		}
		addrsList, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrsList {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}
			addrs = append(addrs, ip.String())
		}
	}
	return addrs
}

// detectTotalMemory detects the total system memory in bytes
func detectTotalMemory() (uint64, error) {
	switch runtime.GOOS {
	case "linux":
		return detectLinuxMemory()
	case "darwin":
		return detectDarwinMemory()
	case "windows":
		return detectWindowsMemory()
	default:
		// Fallback using runtime.MemStats (less accurate but portable)
		return detectFallbackMemory(), nil
	}
}

// detectLinuxMemory reads memory from /proc/meminfo on Linux
func detectLinuxMemory() (uint64, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				memKB, err := strconv.ParseUint(fields[1], 10, 64)
				if err != nil {
					return 0, err
				}
				return memKB * 1024, nil // Convert KB to bytes
			}
		}
	}

	return 0, fmt.Errorf("could not find MemTotal in /proc/meminfo")
}

// detectDarwinMemory detects memory on macOS using sysctl
func detectDarwinMemory() (uint64, error) {
	// Try to read from sysctl hw.memsize
	// This is a simplified implementation - in practice you might want to use cgo or exec sysctl
	return detectFallbackMemory(), nil
}

// detectWindowsMemory detects memory on Windows
func detectWindowsMemory() (uint64, error) {
	// This would require Windows-specific API calls
	// For now, use fallback method
	return detectFallbackMemory(), nil
}

// detectFallbackMemory provides a fallback memory detection using runtime
func detectFallbackMemory() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// This is not accurate for total system memory, but provides a reasonable estimate
	// In practice, you might want to multiply by a factor or use other heuristics
	return m.Sys * 4 // Rough estimate: multiply heap by 4 to estimate system memory
}

// detectOSType determines the operating system type
func detectOSType() OSType {
	switch runtime.GOOS {
	case "linux":
		return OSLinux
	case "windows":
		return OSWindows
	case "darwin":
		return OSMac
	default:
		return OSLinux // Default to Linux for unknown systems
	}
}

// detectPostgreSQLVersion detects PostgreSQL version from environment variables
func detectPostgreSQLVersion() (float64, error) {
	// Check TARGET_VERSION environment variable first
	if targetVersion := os.Getenv("TARGET_VERSION"); targetVersion != "" {
		if version, err := strconv.ParseFloat(targetVersion, 64); err == nil {
			return version, nil
		}
	}

	// Check PG_VERSION environment variable
	if pgVersion := os.Getenv("PG_VERSION"); pgVersion != "" {
		if version, err := strconv.ParseFloat(pgVersion, 64); err == nil {
			return version, nil
		}
	}

	// Check POSTGRES_VERSION environment variable
	if pgVersion := os.Getenv("POSTGRES_VERSION"); pgVersion != "" {
		if version, err := strconv.ParseFloat(pgVersion, 64); err == nil {
			return version, nil
		}
	}

	return 0, fmt.Errorf("PostgreSQL version not found in environment variables")
}

// detectDiskType attempts to detect the disk type
func detectDiskType() (DiskType, error) {
	// This is a simplified detection - real implementation might:
	// - Check /sys/block/*/queue/rotational on Linux
	// - Use system tools to detect SSD vs HDD
	// - Check for NVMe devices
	// - Detect SAN storage

	switch runtime.GOOS {
	case "linux":
		return detectLinuxDiskType()
	default:
		// For now, assume SSD for non-Linux systems
		return DiskSSD, nil
	}
}

// detectLinuxDiskType detects disk type on Linux systems
func detectLinuxDiskType() (DiskType, error) {
	// Try to read rotational flag for the main disk
	// This is a simplified implementation
	rotationalFile := "/sys/block/sda/queue/rotational"

	data, err := os.ReadFile(rotationalFile)
	if err != nil {
		// If we can't read the file, assume SSD (modern default)
		return DiskSSD, nil
	}

	rotational := strings.TrimSpace(string(data))
	if rotational == "0" {
		return DiskSSD, nil
	}

	return DiskHDD, nil
}

// EffectiveCPUCount returns the effective CPU count for tuning
// Uses container CPU limit if available, otherwise system CPUs
func (si *SystemInfo) EffectiveCPUCount() int {
	if si.Container.CPUs > 0 {
		return si.Container.CPUs
	}
	return si.System.CPUs
}

// EffectiveMemory returns the effective memory for tuning
// Uses container memory limit if available, otherwise system memory
func (si *SystemInfo) EffectiveMemory() uint64 {
	if si.Container.Memory > 0 {
		return si.Container.Memory
	}
	return si.System.Memory
}

// TotalMemoryKB returns effective memory in KB
func (si *SystemInfo) TotalMemoryKB() uint64 {
	return si.EffectiveMemory() / 1024
}

// TotalMemoryGB returns effective memory in GB
func (si *SystemInfo) TotalMemoryGB() float64 {
	memGB := float64(si.EffectiveMemory()) / (1024 * 1024 * 1024)
	// Ensure we never return 0, minimum 1GB for display purposes
	if memGB < 1.0 {
		return 1.0
	}
	return memGB
}

// TotalMemoryMB returns effective memory in MB
func (si *SystemInfo) TotalMemoryMB() float64 {
	return float64(si.EffectiveMemory()) / (1024 * 1024)
}

// detectContainer detects if running inside a container
func detectContainer() bool {
	// Check for Docker environment
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// Check for Kubernetes environment
	if _, err := os.Stat("/run/secrets/kubernetes.io"); err == nil {
		return true
	}

	// Check for Kubernetes service account environment variables
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true
	}

	// Check cgroup for container indicators
	if checkCgroupForContainer() {
		return true
	}

	return false
}

// checkCgroupForContainer checks cgroup paths for container indicators
func checkCgroupForContainer() bool {
	// Check for cgroup v1
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		content := string(data)
		if strings.Contains(content, "/docker/") ||
			strings.Contains(content, "/kubepods/") ||
			strings.Contains(content, "/k8s.io/") {
			return true
		}
	}

	// Check for cgroup v2
	if data, err := os.ReadFile("/proc/1/mountinfo"); err == nil {
		content := string(data)
		if strings.Contains(content, "/docker/containers/") ||
			strings.Contains(content, "/kubelet/") {
			return true
		}
	}

	return false
}

// detectMemoryWithContainer detects memory considering container limits
func detectMemoryWithContainer() (totalMem uint64, containerMem uint64, err error) {
	// First try gopsutil for more accurate memory detection
	if vmStat, err := mem.VirtualMemory(); err == nil && vmStat.Total > 0 {
		totalMem = vmStat.Total
	} else {
		// Fallback to OS-specific detection
		totalMem, err = detectTotalMemory()
		if err != nil {
			// Default to 1GB when memory detection fails
			totalMem = 1 * 1024 * 1024 * 1024 // 1GB in bytes
		}

		// If memory detection returned 0, also default to 1GB
		if totalMem == 0 {
			totalMem = 1 * 1024 * 1024 * 1024 // 1GB in bytes
		}
	}

	// Try to detect container memory limits
	containerMem = detectContainerMemoryLimit()

	// If we're in a container and have a limit, use the smaller value
	if containerMem > 0 && containerMem < totalMem {
		totalMem = containerMem
	}

	return totalMem, containerMem, nil
}

// detectContainerMemoryLimit detects container memory limits from cgroups
func detectContainerMemoryLimit() uint64 {
	// Try cgroup v2 first (newer systems)
	if limit := readCgroupV2MemoryLimit(); limit > 0 {
		return limit
	}

	// Fallback to cgroup v1
	if limit := readCgroupV1MemoryLimit(); limit > 0 {
		return limit
	}

	return 0
}

// readCgroupV2MemoryLimit reads memory limit from cgroup v2
// Prioritizes memory.high over memory.max for better Kubernetes integration
// Traverses the cgroup hierarchy to find the most restrictive limit
func readCgroupV2MemoryLimit() uint64 {
	// Check if cgroup v2 is available
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err != nil {
		return 0
	}

	// Find the current cgroup path
	cgroupPath := findCgroupV2Path()
	if cgroupPath == "" {
		cgroupPath = "/"
	}

	// Traverse hierarchy from root to current cgroup
	// Check both memory.high and memory.max at each level
	minLimit := uint64(0)

	// Build path components for traversal
	pathParts := strings.Split(strings.Trim(cgroupPath, "/"), "/")
	currentPath := "/sys/fs/cgroup"

	// Check root first
	if limit := readMemoryLimit(currentPath); limit > 0 {
		minLimit = limit
	}

	// Traverse each level of the hierarchy
	for _, part := range pathParts {
		if part == "" {
			continue
		}
		currentPath = filepath.Join(currentPath, part)
		if limit := readMemoryLimit(currentPath); limit > 0 {
			if minLimit == 0 || limit < minLimit {
				minLimit = limit
			}
		}
	}

	return minLimit
}

// findCgroupV2Path finds the cgroup v2 path for the current process
// In cgroup v2, the line format is "0::/path"
func findCgroupV2Path() string {
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		// cgroup v2 unified hierarchy starts with "0::"
		if strings.HasPrefix(line, "0::") {
			return strings.TrimPrefix(line, "0::")
		}
	}

	return ""
}

// readMemoryLimit reads memory limit from a specific cgroup path
// Prioritizes memory.high (soft limit) over memory.max (hard limit)
// This aligns with Kubernetes MemoryQoS feature
func readMemoryLimit(cgroupPath string) uint64 {
	// Try memory.high first (soft limit, better for K8s)
	highFile := filepath.Join(cgroupPath, "memory.high")
	if data, err := os.ReadFile(highFile); err == nil {
		content := strings.TrimSpace(string(data))
		if content != "max" {
			if limit, err := strconv.ParseUint(content, 10, 64); err == nil && limit > 0 {
				return limit
			}
		}
	}

	// Fallback to memory.max (hard limit)
	maxFile := filepath.Join(cgroupPath, "memory.max")
	if data, err := os.ReadFile(maxFile); err == nil {
		content := strings.TrimSpace(string(data))
		if content != "max" {
			if limit, err := strconv.ParseUint(content, 10, 64); err == nil && limit > 0 {
				return limit
			}
		}
	}

	return 0
}

// readCgroupV1MemoryLimit reads memory limit from cgroup v1
func readCgroupV1MemoryLimit() uint64 {
	// First, find the memory cgroup path
	cgroupPath := findMemoryCgroupPath()
	if cgroupPath == "" {
		return 0
	}

	// Try to read memory.limit_in_bytes
	limitFile := filepath.Join("/sys/fs/cgroup/memory", cgroupPath, "memory.limit_in_bytes")
	if data, err := os.ReadFile(limitFile); err == nil {
		content := strings.TrimSpace(string(data))
		if limit, err := strconv.ParseUint(content, 10, 64); err == nil {
			// Check if this is a real limit (not the default huge value)
			// The default cgroup v1 limit is usually 9223372036854775807 (max int64)
			if limit < 9223372036854775807 {
				return limit
			}
		}
	}

	return 0
}

// findMemoryCgroupPath finds the memory cgroup path for the current process
func findMemoryCgroupPath() string {
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.Contains(line, ":memory:") {
			parts := strings.Split(line, ":")
			if len(parts) >= 3 {
				return strings.TrimPrefix(parts[2], "/")
			}
		}
	}

	return ""
}

// detectContainerCPUQuota detects container CPU quota from cgroups
// Returns the CPU quota as a float (e.g., 0.5, 1.5, 2.0) or 0 if unlimited
func detectContainerCPUQuota() float64 {
	// Try cgroup v2 first (newer systems)
	if quota := readCgroupV2CPUQuota(); quota > 0 {
		return quota
	}

	// Fallback to cgroup v1
	if quota := readCgroupV1CPUQuota(); quota > 0 {
		return quota
	}

	return 0
}

// readCgroupV2CPUQuota reads CPU quota from cgroup v2
// Reads /sys/fs/cgroup/cpu.max which contains "quota period" or "max period"
func readCgroupV2CPUQuota() float64 {
	// Check if cgroup v2 is available by looking for cgroup.controllers
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err != nil {
		return 0
	}

	// Try to read cpu.max (cgroup v2)
	data, err := os.ReadFile("/sys/fs/cgroup/cpu.max")
	if err != nil {
		return 0
	}

	content := strings.TrimSpace(string(data))
	fields := strings.Fields(content)
	if len(fields) < 2 {
		return 0
	}

	// Format is "quota period" or "max period"
	quotaStr := fields[0]
	periodStr := fields[1]

	// "max" means unlimited
	if quotaStr == "max" {
		return 0
	}

	quota, err := strconv.ParseInt(quotaStr, 10, 64)
	if err != nil {
		return 0
	}

	period, err := strconv.ParseInt(periodStr, 10, 64)
	if err != nil || period == 0 {
		return 0
	}

	// CPU quota = quota / period
	// Returns fractional CPUs (e.g., 50000/100000 = 0.5)
	return float64(quota) / float64(period)
}

// readCgroupV1CPUQuota reads CPU quota from cgroup v1
// Reads cpu.cfs_quota_us and cpu.cfs_period_us
func readCgroupV1CPUQuota() float64 {
	// Find the CPU cgroup path
	cgroupPath := findCPUCgroupPath()
	if cgroupPath == "" {
		return 0
	}

	// Read cpu.cfs_quota_us
	quotaFile := filepath.Join("/sys/fs/cgroup/cpu", cgroupPath, "cpu.cfs_quota_us")
	quotaData, err := os.ReadFile(quotaFile)
	if err != nil {
		return 0
	}

	quota, err := strconv.ParseInt(strings.TrimSpace(string(quotaData)), 10, 64)
	if err != nil {
		return 0
	}

	// -1 means unlimited
	if quota == -1 {
		return 0
	}

	// Read cpu.cfs_period_us
	periodFile := filepath.Join("/sys/fs/cgroup/cpu", cgroupPath, "cpu.cfs_period_us")
	periodData, err := os.ReadFile(periodFile)
	if err != nil {
		return 0
	}

	period, err := strconv.ParseInt(strings.TrimSpace(string(periodData)), 10, 64)
	if err != nil || period == 0 {
		return 0
	}

	// CPU quota = quota / period
	return float64(quota) / float64(period)
}

// findCPUCgroupPath finds the CPU cgroup path for the current process
func findCPUCgroupPath() string {
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		// cgroup v1: look for ":cpu:" or ":cpu,cpuacct:"
		if strings.Contains(line, ":cpu:") || strings.Contains(line, ":cpu,cpuacct:") {
			parts := strings.Split(line, ":")
			if len(parts) >= 3 {
				return strings.TrimPrefix(parts[2], "/")
			}
		}
	}

	return ""
}
