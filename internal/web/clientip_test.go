package web

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

func loopbackResolver(t *testing.T, extra ...string) *ClientIPResolver {
	t.Helper()
	raw := append([]string{"127.0.0.1/32", "::1/128"}, extra...)
	prefixes := make([]netip.Prefix, 0, len(raw))
	for _, r := range raw {
		p, err := netip.ParsePrefix(r)
		if err != nil {
			t.Fatalf("bad prefix %q: %v", r, err)
		}
		prefixes = append(prefixes, p.Masked())
	}
	return NewClientIPResolver(prefixes)
}

func TestClientIP(t *testing.T) {
	cases := []struct {
		name       string
		remoteAddr string
		forwarded  []string
		trustExtra []string
		want       string
	}{
		{
			// The whole point: deploy/Caddyfile.example proxies to localhost, so
			// without the header every visitor on earth is 127.0.0.1 and shares
			// one rate-limit bucket.
			name:       "behind the local proxy the real visitor wins",
			remoteAddr: "127.0.0.1:54321",
			forwarded:  []string{"203.0.113.7"},
			want:       "203.0.113.7",
		},
		{
			name:       "IPv6 loopback is a proxy too",
			remoteAddr: "[::1]:54321",
			forwarded:  []string{"203.0.113.7"},
			want:       "203.0.113.7",
		},
		{
			// Everything left of the proxy's own entry is client-supplied.
			name:       "a spoofed chain cannot rotate the bucket",
			remoteAddr: "127.0.0.1:54321",
			forwarded:  []string{"1.1.1.1, 2.2.2.2, 203.0.113.7"},
			want:       "203.0.113.7",
		},
		{
			name:       "repeated headers are treated as one chain",
			remoteAddr: "127.0.0.1:54321",
			forwarded:  []string{"1.1.1.1", "203.0.113.7"},
			want:       "203.0.113.7",
		},
		{
			// Two trusted hops: skip both and take the first untrusted address.
			name:       "trusted hops on the right are skipped",
			remoteAddr: "127.0.0.1:54321",
			forwarded:  []string{"203.0.113.7, 10.1.2.3, 10.4.5.6"},
			trustExtra: []string{"10.0.0.0/8"},
			want:       "203.0.113.7",
		},
		{
			name:       "a direct client cannot forge the header",
			remoteAddr: "198.51.100.4:33445",
			forwarded:  []string{"203.0.113.7"},
			want:       "198.51.100.4",
		},
		{
			name:       "no header behind the proxy falls back to the peer",
			remoteAddr: "127.0.0.1:54321",
			want:       "127.0.0.1",
		},
		{
			name:       "direct client without header",
			remoteAddr: "198.51.100.4:33445",
			want:       "198.51.100.4",
		},
		{
			name:       "garbage in the header is ignored",
			remoteAddr: "127.0.0.1:54321",
			forwarded:  []string{"not-an-ip"},
			want:       "not-an-ip",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := loopbackResolver(t, tc.trustExtra...)
			req := httptest.NewRequest("POST", "/admin/login", nil)
			req.RemoteAddr = tc.remoteAddr
			for _, f := range tc.forwarded {
				req.Header.Add("X-Forwarded-For", f)
			}
			if got := r.ClientIP(req); got != tc.want {
				t.Errorf("ClientIP = %q; want %q", got, tc.want)
			}
		})
	}
}

// Two visitors arriving through the same proxy must land in different buckets —
// that is the difference between throttling an attacker and locking out the
// operator.
func TestClientIPSeparatesVisitorsBehindOneProxy(t *testing.T) {
	r := loopbackResolver(t)

	seen := map[string]bool{}
	for _, visitor := range []string{"203.0.113.7", "198.51.100.9", "2001:db8::1"} {
		req := httptest.NewRequest("POST", "/admin/login", nil)
		req.RemoteAddr = "127.0.0.1:54321"
		req.Header.Set("X-Forwarded-For", visitor)
		seen[r.ClientIP(req)] = true
	}
	if len(seen) != 3 {
		t.Errorf("visitors collapsed into %d buckets: %v", len(seen), seen)
	}
}

func TestIsTrustedPeer(t *testing.T) {
	r := loopbackResolver(t)
	for addr, want := range map[string]bool{
		"127.0.0.1:1234":    true,
		"[::1]:1234":        true,
		"198.51.100.4:1234": false,
		"10.0.0.1:1234":     false,
		"not-an-address":    false,
	} {
		req := httptest.NewRequest("POST", "/", nil)
		req.RemoteAddr = addr
		if got := r.IsTrustedPeer(req); got != want {
			t.Errorf("IsTrustedPeer(%q) = %v; want %v", addr, got, want)
		}
	}
}

// An empty trust list means no header is ever believed.
func TestNoTrustedProxiesIgnoresForwardedHeader(t *testing.T) {
	r := NewClientIPResolver(nil)
	req := httptest.NewRequest("POST", "/admin/login", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("X-Forwarded-For", "203.0.113.7")

	if got := r.ClientIP(req); got != "127.0.0.1" {
		t.Errorf("ClientIP = %q; want the peer address", got)
	}
}
