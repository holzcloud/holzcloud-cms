//go:build linux

package web

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// FreeBytes reports the free space available on the filesystem holding dir.
//
// A full disk does not only break uploads: sessions are written to the same
// database on every authenticated request, so running out of space breaks
// signing in itself. That is why the floor is checked before writes rather than
// discovered afterwards.
func FreeBytes(dir string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dir, &stat); err != nil {
		return 0, fmt.Errorf("statfs %s: %w", dir, err)
	}
	// Bavail is what an unprivileged process may actually use, which is the
	// number that matters — Bfree includes root's reserve.
	return stat.Bavail * uint64(stat.Bsize), nil
}

// WriteProbe creates and removes a file in dir, which is how a filesystem that
// was silently remounted read-only becomes visible.
func WriteProbe(dir string) error {
	path := filepath.Join(dir, ".readyz")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := f.WriteString("ok")
	closeErr := f.Close()
	_ = os.Remove(path)
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
