package version

import (
	"fmt"
	"runtime"
)

// Build information. Populated at build-time via -ldflags
var (
	// Version is the semantic version (e.g., "v1.0.0")
	Version = "dev"

	// GitCommit is the git commit hash
	GitCommit = "unknown"

	// BuildTime is the build timestamp
	BuildTime = "unknown"

	// GitDirty indicates if there were uncommitted changes
	GitDirty = ""
)

// GetVersion returns the version string similar to Foundry's format:
// roamie 0.1.0 (abc1234 2025-11-14T21:51:00Z)
func GetVersion(name string) string {
	dirty := ""
	if GitDirty == "true" {
		dirty = "-dirty"
	}

	return fmt.Sprintf("%s %s (%s%s %s)",
		name,
		Version,
		GitCommit,
		dirty,
		BuildTime,
	)
}

// GetVersionInfo returns detailed version information
func GetVersionInfo() string {
	dirty := "clean"
	if GitDirty == "true" {
		dirty = "dirty"
	}

	return fmt.Sprintf(`Version:    %s
Git commit: %s (%s)
Built:      %s
Go version: %s`,
		Version,
		GitCommit,
		dirty,
		BuildTime,
		runtime.Version(),
	)
}
