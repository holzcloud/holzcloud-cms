package web

import (
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"sync"
)

// ClientIPResolver turns a request into the address rate limiting keys on.
//
// Getting this wrong is not a small error in either direction. Ignoring
// X-Forwarded-For entirely means every visitor arrives as 127.0.0.1 behind the
// documented Caddy setup, so the whole internet shares one bucket and ten failed
// logins from a stranger lock the operator out. Believing it unconditionally
// lets any client rotate its bucket per request and defeats the limit outright.
type ClientIPResolver struct {
	trusted []netip.Prefix

	// untrustedPeerNoted makes the log line about a proxy this resolver does
	// not trust appear once, not once per request.
	untrustedPeerNoted sync.Once
}

// NewClientIPResolver builds a resolver trusting the given proxy prefixes.
func NewClientIPResolver(trusted []netip.Prefix) *ClientIPResolver {
	return &ClientIPResolver{trusted: trusted}
}

// ClientIP returns the caller's address as a string.
//
// When the peer is a trusted proxy, X-Forwarded-For is walked from right to
// left and the first address that is not itself a trusted proxy is returned:
// entries to the right were appended by infrastructure we trust, everything
// further left was supplied by the client and cannot be believed. When the peer
// is not trusted, its own address is the only thing worth keying on.
func (r *ClientIPResolver) ClientIP(req *http.Request) string {
	peer := hostOnly(req.RemoteAddr)
	if !r.isTrusted(peer) {
		return peer
	}

	for _, candidate := range reverse(splitForwarded(req.Header.Values("X-Forwarded-For"))) {
		if !r.isTrusted(candidate) {
			return candidate
		}
	}
	// Every hop was a trusted proxy, or the header was absent.
	return peer
}

// IsTrustedPeer reports whether the request arrived through a trusted proxy.
// A request whose client address could not be established beyond the proxy
// should not be able to exhaust a per-client limit for everyone else.
func (r *ClientIPResolver) IsTrustedPeer(req *http.Request) bool {
	return r.isTrusted(hostOnly(req.RemoteAddr))
}

func (r *ClientIPResolver) isTrusted(host string) bool {
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, prefix := range r.trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// splitForwarded flattens possibly repeated X-Forwarded-For headers into the
// individual addresses they carry, in wire order.
func splitForwarded(values []string) []string {
	var out []string
	for _, value := range values {
		for _, part := range splitComma(value) {
			if host := hostOnly(part); host != "" {
				out = append(out, host)
			}
		}
	}
	return out
}

func splitComma(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			part := trimSpace(s[start:i])
			if part != "" {
				out = append(out, part)
			}
			start = i + 1
		}
	}
	return out
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

func reverse(in []string) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[len(in)-1-i] = v
	}
	return out
}

// hostOnly strips a port from an address if one is present, and brackets from
// a bare IPv6 literal.
func hostOnly(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	if len(addr) > 1 && addr[0] == '[' && addr[len(addr)-1] == ']' {
		return addr[1 : len(addr)-1]
	}
	return addr
}

// throttleKeyMax bounds the key of an address that does not parse, so a value
// chosen by the client cannot become an unbounded map key.
const throttleKeyMax = 64

// ThrottleKey is the key every throttle counts a client under (PWD-02).
//
// An IPv4 address is its own key. An IPv6 address is reduced to its /64,
// because a single subscriber is handed a whole /64 and would otherwise have
// 2^64 budgets to spend. A port, brackets, an IPv4-mapped form and a zone are
// dropped first. What does not parse as an address is cut to 64 bytes.
func ThrottleKey(ip string) string {
	addr, err := netip.ParseAddr(hostOnly(ip))
	if err != nil {
		if len(ip) > throttleKeyMax {
			return ip[:throttleKeyMax]
		}
		return ip
	}
	addr = addr.Unmap().WithZone("")
	if addr.Is4() {
		return addr.String()
	}
	return netip.PrefixFrom(addr, 64).Masked().String()
}

// ThrottleKey is the key the request is counted under, and whether the client
// address it was made from names a visitor at all.
//
// A nil resolver uses the peer address. public is false for loopback, private,
// unspecified and link-local addresses and for anything that does not parse:
// those are what a reverse proxy looks like when HOLZCLOUD_TRUSTED_PROXIES does
// not name it, and a throttle on them would be one budget for every visitor.
// The public page gate skips its client axis then (it keeps the page's own
// budget); the admin throttles key them anyway, because an operator on
// loopback or a LAN must still be throttled.
//
// It logs once per resolver when the peer itself is a private or loopback
// address that the trusted proxies do not cover.
func (r *ClientIPResolver) ThrottleKey(req *http.Request) (key string, public bool) {
	ip := hostOnly(req.RemoteAddr)
	if r != nil {
		r.noteUntrustedPeer(ip)
		ip = r.ClientIP(req)
	}
	return ThrottleKey(ip), isPublicAddress(ip)
}

// noteUntrustedPeer says once, in the log, that visitors are arriving as one
// address because the proxy in front of this program is not trusted.
func (r *ClientIPResolver) noteUntrustedPeer(peer string) {
	addr, err := netip.ParseAddr(peer)
	if err != nil {
		return
	}
	addr = addr.Unmap().WithZone("")
	if !(addr.IsLoopback() || addr.IsPrivate()) || r.isTrusted(peer) {
		return
	}
	r.untrustedPeerNoted.Do(func() {
		slog.Warn("requests arrive from a private or loopback address that HOLZCLOUD_TRUSTED_PROXIES does not cover; "+
			"every visitor then shares this one address and one throttle budget. If a reverse proxy is in front, add it to HOLZCLOUD_TRUSTED_PROXIES",
			"peer", peer)
	})
}

// isPublicAddress reports whether ip is an address a visitor can have: not
// loopback, private, unspecified or link-local, and readable at all.
func isPublicAddress(ip string) bool {
	addr, err := netip.ParseAddr(hostOnly(ip))
	if err != nil {
		return false
	}
	addr = addr.Unmap().WithZone("")
	return !(addr.IsLoopback() || addr.IsPrivate() || addr.IsUnspecified() || addr.IsLinkLocalUnicast())
}
