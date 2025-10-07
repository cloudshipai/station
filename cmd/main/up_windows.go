//go:build windows

package main

import (
	"os"
)

// getDockerGroupID is not supported on Windows
func getDockerGroupID(stat os.FileInfo) int {
	return 0
}
