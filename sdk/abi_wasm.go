//go:build wasip1

package plugin

import "unsafe"

// This file is the whole unsafe surface of a plugin. Everything else in the
// package is ordinary Go, and an author writes ordinary Go.

//go:wasmimport holzcloud hc_call
func hcCall(opPtr, opLen, argPtr, argLen, outPtr, outCap uint32) uint64

// Status codes the host answers with.
const (
	statusOK     = 0
	statusError  = 1
	statusShort  = 2
	statusDenied = 3
)

// incoming is the buffer the host writes a call into.
//
// A package variable, not a local, for two reasons. It has to stay reachable
// while the host writes into it — a slice that fell out of scope the moment
// hc_alloc returned could be collected, and the host would be writing into
// memory the guest no longer owns, which surfaces as a corrupted payload far
// from its cause. And keeping the slice means the input is read back as a
// slice rather than by rebuilding a pointer from an integer, which is the one
// conversion Go does not allow and vet is right to refuse.
//
// One buffer suffices because the host puts the hook name and the payload in
// one allocation, hook first.
var incoming []byte

//go:wasmexport hc_alloc
func hcAlloc(n int32) int32 {
	if n <= 0 {
		return 0
	}
	incoming = make([]byte, n)
	return int32(ptrOf(unsafe.SliceData(incoming)))
}

// outgoing holds the answer while the host reads it, for the same reason.
var outgoing []byte

//go:wasmexport hc_handle
func hcHandle(ptr, hookLen, totalLen int32) uint64 {
	hook, payload := split(hookLen, totalLen)

	res, err := dispatch(hook, payload)
	if err != nil {
		// The error travels as a log line rather than as a return value: a
		// hook's answer has a shape the host parses, and an error squeezed
		// into it would be indistinguishable from a plugin that meant it.
		Log("error", hook+": "+err.Error())
		return 0
	}
	if len(res) == 0 {
		return 0
	}
	outgoing = res
	return pack(ptrOf(unsafe.SliceData(outgoing)), uint32(len(outgoing)))
}

// split cuts the incoming buffer into the hook name and the payload.
//
// The lengths are checked against what was actually allocated rather than
// trusted: a mismatch would otherwise be a read past the end of the slice, and
// a panic inside a hook is a plugin that looks broken for a reason nobody can
// see.
func split(hookLen, totalLen int32) (string, []byte) {
	if hookLen < 0 || totalLen < hookLen || int(totalLen) > len(incoming) {
		return "", nil
	}
	return string(incoming[:hookLen]), incoming[hookLen:totalLen]
}

func pack(ptr, n uint32) uint64 { return uint64(ptr)<<32 | uint64(n) }

// hostCall performs one call into the host.
//
// The buffer starts small and grows only when the host says the answer did not
// fit. Most answers are a few hundred bytes, so the common case is one call.
func hostCall(op string, arg []byte) ([]byte, error) {
	out := make([]byte, 4096)
	for {
		r := hcCall(
			ptrOf(unsafe.StringData(op)), uint32(len(op)),
			ptrOfBytes(arg), uint32(len(arg)),
			ptrOf(unsafe.SliceData(out)), uint32(len(out)))

		status, n := uint32(r>>32), uint32(r)
		switch status {
		case statusOK:
			if n == 0 {
				return nil, nil
			}
			// Copied: out is reused by the next call.
			res := make([]byte, n)
			copy(res, out[:n])
			return res, nil
		case statusShort:
			out = make([]byte, n)
			continue
		case statusDenied:
			return nil, ErrDenied
		default:
			return nil, hostError(string(out[:n]))
		}
	}
}

// hostError is what the host said went wrong.
type hostError string

func (e hostError) Error() string { return string(e) }

// ptrOf is the address of a byte in linear memory. Pointer to integer, which
// is the direction Go allows; the other way round never happens in this file.
func ptrOf(p *byte) uint32 {
	if p == nil {
		return 0
	}
	return uint32(uintptr(unsafe.Pointer(p)))
}

func ptrOfBytes(b []byte) uint32 {
	if len(b) == 0 {
		return 0
	}
	return ptrOf(unsafe.SliceData(b))
}
