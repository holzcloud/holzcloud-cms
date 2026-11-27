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

// argon2Key is the key-derivation function, a variable so a test can count and
// hold derivations. derive is the only place it is called.
var argon2Key = argon2.IDKey

// derive runs one Argon2id derivation inside the gate. privileged picks the
// longer wait: hashing a new password and a signed-in person re-entering their
// own are privileged, a stranger's check is anonymous. It returns ErrBusy,
// unwrapped, when the gate stayed full; no key is derived then.
func derive(privileged bool, password string, salt []byte, p Argon2Params) ([]byte, error) {
	g := argon2Gate.Load()
	wait := g.anonymous
	if privileged {
		wait = g.privilege
	}
	release, err := g.Acquire(wait)
	if err != nil {
		return nil, err
	}
	defer release()
	return argon2Key([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength), nil
}

// HashPassword returns a PHC-format Argon2id hash string. It is a privileged
// derivation: a new password is only ever set by somebody entitled to set it.
func HashPassword(password string, p Argon2Params) (string, error) {
	salt := make([]byte, p.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	hash, err := derive(true, password, salt, p)
	if err != nil {
		return "", err
	}
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
// Every parameter falls back to the default, and not only the key length.
//
// argon2.IDKey panics on zero rounds — "argon2: number of rounds too small" —
// and this is called on the "no such address" branch of a login. A panic there
// is a 500 where a known address gets a 200, which tells an attacker exactly
// what the timing would have told them, instantly and without measuring
// anything. The defence would have become the oracle it was written against.
//
// config.Load cannot produce such a struct: it floors the memory at 8 KB and
// refuses zero iterations and zero parallelism, collecting all three as errors
// rather than quietly defaulting. This is for every other caller — a partially
// filled struct is easy to write and the compiler has nothing to say about it.
//
// It returns nil, or ErrBusy when the gate refused: the caller then answers
// "busy" exactly as for a known address.
func VerifyDummyPassword(password string, p Argon2Params) error {
	if p.KeyLength == 0 {
		p.KeyLength = DefaultParams.KeyLength
	}
	if p.Iterations == 0 {
		p.Iterations = DefaultParams.Iterations
	}
	if p.Memory == 0 {
		p.Memory = DefaultParams.Memory
	}
	if p.Parallelism == 0 {
		p.Parallelism = DefaultParams.Parallelism
	}
	hash, err := derive(false, password, dummySalt, p)
	if err != nil {
		return err
	}
	// Compare against itself so the compiler cannot elide the derivation.
	_ = subtle.ConstantTimeCompare(hash, hash)
	return nil
}

// MinPasswordLength is the minimum accepted password length, enforced anywhere
// a password is set: initial setup, user creation and password change.
const MinPasswordLength = 8

// VerifyPassword checks a password against a PHC-format Argon2id hash, as an
// anonymous caller: it waits briefly for a slot and returns ErrBusy when none
// is free. Parameters are parsed from the hash so old hashes remain verifiable
// after param changes.
func VerifyPassword(password, encoded string) (bool, error) {
	return verify(false, password, encoded)
}

// VerifyOwnPassword is VerifyPassword for a signed-in person re-entering their
// own password (a confirmation, a password change). It waits as long as a
// privileged action does. The password is compared exactly as typed.
func VerifyOwnPassword(password, encoded string) (bool, error) {
	return verify(true, password, encoded)
}

func verify(privileged bool, password, encoded string) (bool, error) {
	p, salt, expectedHash, err := parseHash(encoded)
	if err != nil {
		return false, err
	}
	hash, err := derive(privileged, password, salt, p)
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(hash, expectedHash) == 1, nil
}

// parseHash reads a PHC-format Argon2id string:
// $argon2id$v=19$m=65536,t=1,p=2$salt$hash
func parseHash(encoded string) (p Argon2Params, salt, expectedHash []byte, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return p, nil, nil, fmt.Errorf("invalid hash format: expected 6 parts, got %d", len(parts))
	}
	if parts[1] != "argon2id" {
		return p, nil, nil, fmt.Errorf("invalid hash format: unsupported algorithm %q", parts[1])
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return p, nil, nil, fmt.Errorf("invalid hash format: %w", err)
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Iterations, &p.Parallelism); err != nil {
		return p, nil, nil, fmt.Errorf("invalid hash format: %w", err)
	}
	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return p, nil, nil, fmt.Errorf("invalid hash format: decode salt: %w", err)
	}
	expectedHash, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return p, nil, nil, fmt.Errorf("invalid hash format: decode hash: %w", err)
	}
	p.KeyLength = uint32(len(expectedHash))
	return p, salt, expectedHash, nil
}
