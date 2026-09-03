//go:build !linux

package web

import (
	"os"
	"path/filepath"
)

// FreeBytes is unavailable off Linux: the figure comes from a Linux-only
// syscall, and the deployment target is Linux. Reporting a huge number keeps
// development on other platforms working without pretending to know the real
// figure.
func FreeBytes(dir string) (uint64, error) {
	if _, err := os.Stat(dir); err != nil {
		return 0, err
	}
	return 1 << 62, nil
}

// WriteProbe creates and removes a file in dir.
func WriteProbe(dir string) error {
	path := filepath.Join(dir, ".readyz")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	closeErr := f.Close()
	_ = os.Remove(path)
	return closeErr
}
