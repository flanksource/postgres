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

// SystemInfo contains detected system information
type SystemInfo struct {
	IPAddresses []string `json:"ip_addresses,omitempty"`

	// TotalMemoryBytes is the total system memory in bytes
	TotalMemoryBytes uint64 `json:"total_memory_bytes,omitempty" pretty:"format=bytes"`

	// CPUCount is the number of logical CPUs
	CPUCount int `json:"cpu_count,omitempty"`

	// OSType is the operating system type
	OSType OSType `json:"os_type,omitempty"`

	// PostgreSQLVersion is the PostgreSQL major version
	PostgreSQLVersion float64 `json:"postgres_version,omitempty"`

	// DiskType is the detected or assumed disk type
	DiskType DiskType `json:"disk_type,omitempty"`

	// IsContainer indicates if running inside a container
	IsContainer bool `json:"is_container,omitempty"`

	// ContainerMemory is the container memory limit in bytes (0 if not limited)
	ContainerMemory uint64 `json:"container_memory,omitempty" pretty:"format=bytes"`
}

// DetectSystemInfo automatically detects system information
func DetectSystemInfo() (*SystemInfo, error) {
	info := &SystemInfo{}

	// Detect if running in a container
	info.IsContainer = detectContainer()

	// Detect memory - use container limits if available (defaults to 1GB if not detected)
	totalMem, containerMem, _ := detectMemoryWithContainer()
	info.TotalMemoryBytes = totalMem
	info.ContainerMemory = containerMem

	// Detect CPU count
	info.CPUCount = runtime.NumCPU()

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

// TotalMemoryGB returns total memory in GB
func (si *SystemInfo) TotalMemoryGB() float64 {
	memGB := float64(si.TotalMemoryBytes) / (1024 * 1024 * 1024)
	// Ensure we never return 0, minimum 1GB for display purposes
	if memGB < 1.0 {
		return 1.0
	}
	return memGB
}

// TotalMemoryMB returns total memory in MB
func (si *SystemInfo) TotalMemoryMB() float64 {
	return float64(si.TotalMemoryBytes) / (1024 * 1024)
}

// TotalMemoryKB returns total memory in KB
func (si *SystemInfo) TotalMemoryKB() uint64 {
	return si.TotalMemoryBytes / 1024
}

// String returns a human-readable representation of the system info
func (si *SystemInfo) String() string {
	containerInfo := ""
	if si.IsContainer {
		if si.ContainerMemory > 0 {
			containerMemGB := float64(si.ContainerMemory) / (1024 * 1024 * 1024)
			containerInfo = fmt.Sprintf(", Container: %.2f GB", containerMemGB)
		} else {
			containerInfo = ", Container: unlimited"
		}
	}
	return fmt.Sprintf("SystemInfo{Memory: %.2f GB, CPUs: %d, OS: %s, PostgreSQL: %.1f, Disk: %s%s}",
		si.TotalMemoryGB(), si.CPUCount, si.OSType, si.PostgreSQLVersion, si.DiskType, containerInfo)
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
func readCgroupV2MemoryLimit() uint64 {
	// Check if cgroup v2 is available
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err != nil {
		return 0
	}

	// Try to read memory.max (cgroup v2)
	if data, err := os.ReadFile("/sys/fs/cgroup/memory.max"); err == nil {
		content := strings.TrimSpace(string(data))
		if content != "max" {
			if limit, err := strconv.ParseUint(content, 10, 64); err == nil {
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
