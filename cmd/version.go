package main

import (
	"fmt"
	"runtime"
)

var (
	// Version is the semantic version (set via ldflags)
	Version = "dev"
	// GitCommit is the git commit hash (set via ldflags)
	GitCommit = "unknown"
	// BuildDate is the build timestamp (set via ldflags)
	BuildDate = "unknown"
)

// GetVersionInfo returns formatted version information
func GetVersionInfo() string {
	return fmt.Sprintf(`postgres-cli version: %s
Git commit: %s
Build date: %s
Go version: %s
OS/Arch: %s/%s`, Version, GitCommit, BuildDate, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
