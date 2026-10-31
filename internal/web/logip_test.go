package web

import (
	"math/rand"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func TestTruncateIP(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"203.0.113.77", "203.0.113.0"},
		{"203.0.113.77:51234", "203.0.113.0"},
		{"255.255.255.255", "255.255.255.0"},
		{"127.0.0.1", "127.0.0.0"},
		{"2001:db8:abcd:1234:5678:9abc:def0:1234", "2001:db8:abcd::"},
		{"[2001:db8:abcd:1234::1]:443", "2001:db8:abcd::"},
		{"::ffff:203.0.113.77", "203.0.113.0"},
		{"fe80::1%eth0", "fe80::"},
		{"::1", "::"},
	} {
		if got := TruncateIP(tc.in); got != tc.want {
			t.Errorf("TruncateIP(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// A value that is not an address is never echoed: a trusted proxy that forwards
// a hostile X-Forwarded-For must not be able to write a line into the journal.
func TestTruncateIPNeverEchoesInput(t *testing.T) {
	for _, in := range []string{
		"",
		"unknown",
		"not an ip",
		"203.0.113.777",
		"1.2.3",
		"203.0.113.77, 10.0.0.1",
		"203.0.113.77\nlevel=ERROR msg=forged",
	} {
		got := TruncateIP(in)
		if got != UnknownIP {
			t.Errorf("TruncateIP(%q) = %q, want %q", in, got, UnknownIP)
		}
		if strings.ContainsAny(got, "\r\n") {
			t.Errorf("TruncateIP(%q) = %q holds a line break", in, got)
		}
	}
	forged := "203.0.113.77\nlevel=ERROR msg=forged"
	if got := TruncateIP(forged); strings.Contains(got, "forged") || strings.Contains(got, "203") {
		t.Errorf("TruncateIP(%q) = %q echoes part of its input", forged, got)
	}
}

func TestTruncateIPIsIdempotentAndMasks(t *testing.T) {
	rng := rand.New(rand.NewSource(30))
	for i := 0; i < 300; i++ {
		var in netip.Addr
		bits := LogIPv4Prefix
		if i%2 == 0 {
			var b [4]byte
			rng.Read(b[:])
			in = netip.AddrFrom4(b)
		} else {
			var b [16]byte
			rng.Read(b[:])
			in = netip.AddrFrom16(b)
			bits = LogIPv6Prefix
			if in.Is4In6() {
				bits = LogIPv4Prefix
			}
		}

		got := TruncateIP(in.String())
		addr, err := netip.ParseAddr(got)
		if err != nil {
			t.Fatalf("TruncateIP(%q) = %q does not parse: %v", in, got, err)
		}
		if again := TruncateIP(got); again != got {
			t.Errorf("TruncateIP is not idempotent for %q: %q then %q", in, got, again)
		}
		if want := netip.PrefixFrom(addr, bits).Masked().Addr(); addr != want {
			t.Errorf("TruncateIP(%q) = %q keeps host bits beyond /%d", in, got, bits)
		}
	}
}

// Truncation belongs to the log. The resolver the rate limit keys on still
// returns the whole address.
func TestClientIPStaysWhole(t *testing.T) {
	resolver := NewClientIPResolver([]netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")})
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:1"
	req.Header.Set("X-Forwarded-For", "203.0.113.77")

	got := resolver.ClientIP(req)
	if got != "203.0.113.77" {
		t.Fatalf("ClientIP = %q, want the whole address 203.0.113.77", got)
	}
	if TruncateIP(got) == got {
		t.Errorf("TruncateIP(%q) did not shorten the address", got)
	}
}
