//go:build unix

package main

import (
	"os"
	"syscall"
)

// getDockerGroupID extracts the group ID from file stat (Unix only)
func getDockerGroupID(stat os.FileInfo) int {
	if sysStat, ok := stat.Sys().(*syscall.Stat_t); ok {
		return int(sysStat.Gid)
	}
	return 0
}
