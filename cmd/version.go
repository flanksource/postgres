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
	// Shorten git commit to first 7 chars
	shortCommit := GitCommit
	if len(shortCommit) > 7 {
		shortCommit = shortCommit[:7]
	}

	return fmt.Sprintf("postgres-cli v%s (git:%s, built:%s, %s, %s/%s)",
		Version,
		shortCommit,
		BuildDate,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH)
}
