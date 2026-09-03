package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2Params holds the parameters for Argon2id hashing.
type Argon2Params struct {
	Memory      uint32 // KB
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// DefaultParams are sized for a small single-tenant server: 64 MB, one
// iteration, two threads. All three are env-tunable via
// HOLZCLOUD_ARGON2_MEMORY, _ITERATIONS and _PARALLELISM — raise them when the
// machine has room to spare.
var DefaultParams = Argon2Params{
	Memory:      64 * 1024, // 64 MB
	Iterations:  1,
	Parallelism: 2,
	SaltLength:  16,
	KeyLength:   32,
}

// HashPassword returns a PHC-format Argon2id hash string.
func HashPassword(password string, p Argon2Params) (string, error) {
	salt := make([]byte, p.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.Memory, p.Iterations, p.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash)), nil
}

// dummySalt is a fixed salt used only by VerifyDummyPassword. It never protects
// a real secret, so a constant is fine.
var dummySalt = []byte("holzcloud-dummy!")

// VerifyDummyPassword performs an Argon2id derivation with the same cost as a
// real verification and discards the result.
//
// Call it on the "user not found" path of a login so the response time does not
// distinguish an unknown email from a wrong password.
func VerifyDummyPassword(password string, p Argon2Params) {
	keyLen := p.KeyLength
	if keyLen == 0 {
		keyLen = DefaultParams.KeyLength
	}
	hash := argon2.IDKey([]byte(password), dummySalt, p.Iterations, p.Memory, p.Parallelism, keyLen)
	// Compare against itself so the compiler cannot elide the derivation.
	_ = subtle.ConstantTimeCompare(hash, hash)
}

// MinPasswordLength is the minimum accepted password length, enforced anywhere
// a password is set: initial setup, user creation and password change.
const MinPasswordLength = 8

// VerifyPassword checks a password against a PHC-format Argon2id hash.
// Parameters are parsed from the hash so old hashes remain verifiable after param changes.
func VerifyPassword(password, encoded string) (bool, error) {
	// PHC format: $argon2id$v=19$m=65536,t=1,p=2$salt$hash
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return false, fmt.Errorf("invalid hash format: expected 6 parts, got %d", len(parts))
	}
	if parts[1] != "argon2id" {
		return false, fmt.Errorf("invalid hash format: unsupported algorithm %q", parts[1])
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, fmt.Errorf("invalid hash format: %w", err)
	}

	var p Argon2Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Iterations, &p.Parallelism); err != nil {
		return false, fmt.Errorf("invalid hash format: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("invalid hash format: decode salt: %w", err)
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("invalid hash format: decode hash: %w", err)
	}
	p.KeyLength = uint32(len(expectedHash))

	hash := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)
	return subtle.ConstantTimeCompare(hash, expectedHash) == 1, nil
}
