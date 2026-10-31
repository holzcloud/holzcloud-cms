package web

import "net/netip"

const (
	// LogIPv4Prefix is the number of leading bits of an IPv4 client address
	// that the request log keeps; the rest is cut to zero.
	LogIPv4Prefix = 24

	// LogIPv6Prefix is the number of leading bits of an IPv6 client address
	// that the request log keeps; the rest is cut to zero.
	LogIPv6Prefix = 48

	// LogRetentionDays is how long the journal holds the request log. It is the
	// number the journald drop-in in deploy/ enforces and the privacy text
	// states; a test holds the three together.
	LogRetentionDays = 14

	// UnknownIP is what the request log holds when the client address cannot be
	// read as an address. The unreadable value itself is never logged.
	UnknownIP = "unknown"
)

// TruncateIP cuts a client address to its network: /24 for IPv4, /48 for IPv6,
// written as the network address without a suffix (203.0.113.77 becomes
// 203.0.113.0). A /24 or /48 still shows the network, which is enough to find a
// fault, and no longer names a household.
//
// A value that is not an address (empty, text, a forged line with a newline
// that a trusted proxy forwarded) comes back as UnknownIP, never as any part
// of the input, so it cannot write a line of its choice into the journal.
//
// Rate limiting uses the whole address from ClientIPResolver.ClientIP, in
// memory, and never this function. TruncateIP is applied only where an address
// is written to the request log.
func TruncateIP(raw string) string {
	addr, err := netip.ParseAddr(hostOnly(raw))
	if err != nil {
		return UnknownIP
	}
	addr = addr.Unmap().WithZone("")
	bits := LogIPv6Prefix
	if addr.Is4() {
		bits = LogIPv4Prefix
	}
	return netip.PrefixFrom(addr, bits).Masked().Addr().String()
}
