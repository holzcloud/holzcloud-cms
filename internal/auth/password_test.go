package auth

import (
	"strings"
	"testing"
)

func TestPasswordHashFormat(t *testing.T) {
	hash, err := HashPassword("testpassword", DefaultParams)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$") {
		t.Errorf("hash should start with $argon2id$v=19$, got %s", hash)
	}
}

func TestPasswordVerifyCorrect(t *testing.T) {
	hash, err := HashPassword("correcthorse", DefaultParams)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	ok, err := VerifyPassword("correcthorse", hash)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Error("VerifyPassword should return true for correct password")
	}
}

func TestPasswordVerifyWrong(t *testing.T) {
	hash, err := HashPassword("correcthorse", DefaultParams)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	ok, err := VerifyPassword("wrongpassword", hash)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if ok {
		t.Error("VerifyPassword should return false for wrong password")
	}
}

func TestPasswordVerifyParsesParams(t *testing.T) {
	// Hash with non-default params
	custom := Argon2Params{
		Memory:      32 * 1024,
		Iterations:  2,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	}
	hash, err := HashPassword("mypassword", custom)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	// Verify should parse params from hash, not use DefaultParams
	ok, err := VerifyPassword("mypassword", hash)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Error("VerifyPassword should work with non-default params parsed from hash")
	}
}

func TestPasswordDifferentSalts(t *testing.T) {
	h1, err := HashPassword("samepassword", DefaultParams)
	if err != nil {
		t.Fatalf("HashPassword 1: %v", err)
	}
	h2, err := HashPassword("samepassword", DefaultParams)
	if err != nil {
		t.Fatalf("HashPassword 2: %v", err)
	}
	if h1 == h2 {
		t.Error("two hashes of the same password should differ (different salts)")
	}
}

func TestPasswordInvalidHash(t *testing.T) {
	_, err := VerifyPassword("pass", "notahash")
	if err == nil {
		t.Error("VerifyPassword should return error for invalid hash format")
	}
}
