package version

import (
	"fmt"
	"runtime"
)

// Build-time variables injected by ldflags
var (
	Version   = "dev"     // Version of the application
	BuildTime = "unknown" // Build timestamp
)

// BuildInfo contains comprehensive build information
type BuildInfo struct {
	Version   string `json:"version"`
	BuildTime string `json:"build_time"`
	GoVersion string `json:"go_version"`
	GoArch    string `json:"go_arch"`
	GoOS      string `json:"go_os"`
	Compiler  string `json:"compiler"`
}

// GetBuildInfo returns comprehensive build information
func GetBuildInfo() BuildInfo {
	return BuildInfo{
		Version:   Version,
		BuildTime: BuildTime,
		GoVersion: runtime.Version(),
		GoArch:    runtime.GOARCH,
		GoOS:      runtime.GOOS,
		Compiler:  runtime.Compiler,
	}
}

// GetVersion returns the application version
func GetVersion() string {
	return Version
}

// GetVersionString returns a formatted version string
func GetVersionString() string {
	return Version
}

// GetFullVersionString returns a comprehensive version string
func GetFullVersionString() string {
	return fmt.Sprintf("Station %s\nBuilt: %s\nGo: %s",
		Version, BuildTime, runtime.Version())
}

// IsDevBuild returns true if this is a development build
func IsDevBuild() bool {
	return Version == "dev"
}
