package local

import (
	"fmt"
	"syscall"

	"gos3/internal/s3"
)

// checkDiskSpace checks if there's enough free space on the disk containing dataDir
// to store an object of the given size.
func checkDiskSpace(dataDir string, requiredSize int64) error {
	var stat syscall.Statfs_t

	if err := syscall.Statfs(dataDir, &stat); err != nil {
		return fmt.Errorf("failed to check disk space: %w", err)
	}

	// Available blocks * size per block = available bytes
	availableBytes := int64(stat.Bavail) * int64(stat.Bsize)

	if requiredSize > availableBytes {
		return s3.ErrInsufficientStorage
	}

	return nil
}
