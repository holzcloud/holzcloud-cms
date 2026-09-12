// Package wasmtest decides in one place what happens when a built plugin module
// is missing.
//
// On a contributor's machine a missing .wasm is not a fault: they may have
// checked out only part of the tree, and a test that punishes them for it
// drives them away. On a runner it is one — a test that skips itself reports
// green and checks nothing, and that is exactly the false success nobody
// notices, because it looks like a success.
//
// HOLZCLOUD_TEST_REQUIRE_WASM tells the two cases apart. The three workflows
// that run tests — ci.yml, security.yml and release.yml — set the variable;
// image.yml runs no tests and does not set it. The check is for "not empty" and
// not for the value 1: whoever reproduces a runner's failure locally reaches
// for true or yes, and a strict comparison would silently let them keep
// skipping — the same gap, only one level down.
//
// The package is not a test package, because the five call sites lie in three
// different Go packages and a helper declared in a test file does not reach the
// other two. Nothing that ships imports it, so it contributes nothing to the
// binary.
package wasmtest

import (
	"os"
	"testing"
)

// buildHint names the one command that produces every missing module. It stands
// here once, so that the skipping message and the failing message cannot drift
// apart.
const buildHint = "build it with: go run ./tools/wasm"

// Module reads a built .wasm for a test. path is relative to the test's
// directory.
//
// If the file is missing, HOLZCLOUD_TEST_REQUIRE_WASM decides: set (with any
// non-empty value) makes the test fail, unset makes it skip. Both messages name
// the path, the underlying error and the build hint.
func Module(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err == nil {
		return b
	}
	if os.Getenv("HOLZCLOUD_TEST_REQUIRE_WASM") != "" {
		t.Fatalf("%s is missing and HOLZCLOUD_TEST_REQUIRE_WASM is set: %v\n%s", path, err, buildHint)
	}
	t.Skipf("%s is missing: %v\n%s", path, err, buildHint)
	return nil
}
