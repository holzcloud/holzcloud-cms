package auth

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
)

// LoadOrGenerateCSRFKey reads the CSRF key from dataDir/csrf.key.
// If the file does not exist or has the wrong length, a new 32-byte key
// is generated, written with 0600 permissions, and returned.
func LoadOrGenerateCSRFKey(dataDir string) ([]byte, error) {
	path := filepath.Join(dataDir, "csrf.key")
	key, err := os.ReadFile(path)
	if err == nil && len(key) == 32 {
		return key, nil
	}
	key = make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate csrf key: %w", err)
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, fmt.Errorf("write csrf key: %w", err)
	}
	return key, nil
}
