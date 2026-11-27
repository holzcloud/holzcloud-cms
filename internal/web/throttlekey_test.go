package web

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"
)

func TestThrottleKey(t *testing.T) {
	cases := []struct{ in, want string }{
		{"203.0.113.7", "203.0.113.7"},
		{"203.0.113.7:4711", "203.0.113.7"},
		{"::ffff:203.0.113.7", "203.0.113.7"},
		{"2001:db8:1:2:aaaa::1", "2001:db8:1:2::/64"},
		{"2001:db8:1:2:bbbb::2", "2001:db8:1:2::/64"},
		{"[2001:db8::1]:443", "2001:db8::/64"},
		{"fe80::1%eth0", "fe80::/64"},
	}
	for _, c := range cases {
		if got := ThrottleKey(c.in); got != c.want {
			t.Errorf("ThrottleKey(%q) = %q, want %q", c.in, got, c.want)
		}
	}

	if ThrottleKey("2001:db8:1:3::1") == ThrottleKey("2001:db8:1:2::1") {
		t.Error("two different /64 networks share a key")
	}

	long := make([]byte, 200)
	for i := range long {
		long[i] = 'x'
	}
	if got := ThrottleKey(string(long)); len(got) > 64 {
		t.Errorf("an unparsable value keeps %d bytes, want at most 64", len(got))
	}
}

func resolverFor(t *testing.T, trusted ...string) *ClientIPResolver {
	t.Helper()
	var prefixes []netip.Prefix
	for _, p := range trusted {
		prefixes = append(prefixes, netip.MustParsePrefix(p))
	}
	return NewClientIPResolver(prefixes)
}

func throttleRequest(peer, forwarded string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = peer
	if forwarded != "" {
		req.Header.Set("X-Forwarded-For", forwarded)
	}
	return req
}

func TestResolverThrottleKeyGroupsASlashSixtyFour(t *testing.T) {
	r := resolverFor(t, "127.0.0.1/32")
	a, _ := r.ThrottleKey(throttleRequest("127.0.0.1:5000", "2001:db8:1:2::1"))
	b, _ := r.ThrottleKey(throttleRequest("127.0.0.1:5000", "2001:db8:1:2::ffff"))
	c, _ := r.ThrottleKey(throttleRequest("127.0.0.1:5000", "2001:db8:1:3::1"))
	if a != b {
		t.Errorf("two addresses of one /64 are two keys: %q and %q", a, b)
	}
	if a == c {
		t.Error("two /64 networks share a key")
	}

	for _, c := range []struct {
		forwarded string
		public    bool
	}{
		{"198.51.100.7", true},
		{"2001:db8:1:2::1", true},
		{"10.0.0.5", false},
		{"127.0.0.1", false},
		{"not-an-address", false},
	} {
		_, public := r.ThrottleKey(throttleRequest("127.0.0.1:5000", c.forwarded))
		if public != c.public {
			t.Errorf("public for %q = %v, want %v", c.forwarded, public, c.public)
		}
	}

	var nilResolver *ClientIPResolver
	key, public := nilResolver.ThrottleKey(throttleRequest("198.51.100.9:443", "203.0.113.1"))
	if key != "198.51.100.9" || !public {
		t.Errorf("a nil resolver gave %q, %v; want the peer address, public", key, public)
	}
}

// recordingHandler keeps what slog is asked to write.
type recordingHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *recordingHandler) Handle(_ context.Context, rec slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, rec)
	return nil
}
func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(string) slog.Handler      { return h }

func TestAnUntrustedPrivatePeerIsLoggedOnce(t *testing.T) {
	h := &recordingHandler{}
	previous := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(previous) })

	r := resolverFor(t, "127.0.0.1/32")
	r.ThrottleKey(throttleRequest("172.17.0.1:4000", ""))
	r.ThrottleKey(throttleRequest("172.17.0.1:4001", ""))
	if len(h.records) != 1 {
		t.Fatalf("%d log records for an untrusted private peer, want exactly 1", len(h.records))
	}
	rec := h.records[0]
	if rec.Level != slog.LevelWarn {
		t.Errorf("level = %v, want Warn", rec.Level)
	}
	var peer string
	rec.Attrs(func(a slog.Attr) bool {
		if a.Key == "peer" {
			peer = a.Value.String()
		}
		return true
	})
	if peer != "172.17.0.1" || !strings.Contains(rec.Message, "HOLZCLOUD_TRUSTED_PROXIES") {
		t.Errorf("record names peer %q and %q; want the peer and HOLZCLOUD_TRUSTED_PROXIES", peer, rec.Message)
	}

	// A trusted loopback peer and a public peer say nothing.
	h2 := &recordingHandler{}
	slog.SetDefault(slog.New(h2))
	fresh := resolverFor(t, "127.0.0.1/32")
	fresh.ThrottleKey(throttleRequest("127.0.0.1:4000", "203.0.113.1"))
	fresh.ThrottleKey(throttleRequest("198.51.100.7:4000", ""))
	if len(h2.records) != 0 {
		t.Errorf("%d log records for a trusted or public peer, want none", len(h2.records))
	}
}

// I-6: a link-local peer (fe80::/10, 169.254.0.0/16) is not a visitor either:
// it is a proxy on the same link that HOLZCLOUD_TRUSTED_PROXIES does not name,
// and it is told about like a private or loopback one.
func TestAnUntrustedLinkLocalPeerIsLoggedToo(t *testing.T) {
	for _, peer := range []string{"[fe80::1%eth0]:4000", "169.254.10.20:4000"} {
		h := &recordingHandler{}
		previous := slog.Default()
		slog.SetDefault(slog.New(h))
		r := resolverFor(t, "127.0.0.1/32")
		r.ThrottleKey(throttleRequest(peer, ""))
		r.ThrottleKey(throttleRequest(peer, ""))
		slog.SetDefault(previous)
		if len(h.records) != 1 || h.records[0].Level != slog.LevelWarn {
			t.Errorf("%s: %d log records, want exactly one warning", peer, len(h.records))
		}
	}
}
