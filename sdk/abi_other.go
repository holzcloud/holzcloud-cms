//go:build !wasip1

package plugin

import "errors"

// Off WebAssembly there is no host to call.
//
// The package still builds, and that is the point: a plugin author can write
// ordinary Go tests for their handlers on their own machine, run them with
// plain `go test`, and only reach for the wasm toolchain when they want the
// module. A handler that is pure logic — and most are — never needs the host
// at all.
//
// Tests that do need the host substitute one with SetTestHost.
var errNoHost = errors.New("kein Host: ausserhalb von WebAssembly gibt es nichts aufzurufen")

// testHost stands in for the host in tests.
var testHost func(op string, arg []byte) ([]byte, error)

// SetTestHost installs a stand-in for the host, for tests.
//
// Only present off WebAssembly, so it cannot be called from a real plugin and
// cannot become a way around the permission checks.
func SetTestHost(f func(op string, arg []byte) ([]byte, error)) { testHost = f }

// Dispatch runs a hook against the registered handlers, for tests.
//
// It is the same path the host takes, so a test exercises the real
// marshalling rather than calling the handler directly.
func Dispatch(hook string, payload []byte) ([]byte, error) { return dispatch(hook, payload) }

func hostCall(op string, arg []byte) ([]byte, error) {
	if testHost == nil {
		return nil, errNoHost
	}
	return testHost(op, arg)
}
