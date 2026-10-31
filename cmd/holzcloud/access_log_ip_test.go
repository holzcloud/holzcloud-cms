package main

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// logSink is a buffer that slog may write into from any goroutine.
type logSink struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *logSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *logSink) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// The request log holds the network of a visitor and nothing finer, and it does
// so for the router the server serves, not for the middleware alone.
func TestAccessLogWritesOnlyTheTruncatedAddress(t *testing.T) {
	for _, tc := range []struct {
		name, remote, forwarded string
		want, forbidden         string
	}{
		{"trusted proxy, IPv4", "127.0.0.1:41234", "203.0.113.77", "ip=203.0.113.0 ", "203.0.113.77"},
		{"trusted proxy, IPv6", "127.0.0.1:41234", "2001:db8:abcd:1234::77", "ip=2001:db8:abcd:: ", "1234"},
		{"no proxy", "198.51.100.23:5555", "", "ip=198.51.100.0 ", "198.51.100.23"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sink := &logSink{}
			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(sink, &slog.HandlerOptions{Level: slog.LevelDebug})))
			t.Cleanup(func() { slog.SetDefault(previous) })

			handler, _, _ := testRouterWith(t, routerTweaks{
				clientIP: web.NewClientIPResolver([]netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}),
			})
			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			req.RemoteAddr = tc.remote
			if tc.forwarded != "" {
				req.Header.Set("X-Forwarded-For", tc.forwarded)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("GET /healthz answered %d, want 200", rec.Code)
			}

			out := sink.String()
			if !strings.Contains(out, "msg=request") {
				t.Fatalf("no request record was logged; log: %q", out)
			}
			if !strings.Contains(out, tc.want) {
				t.Errorf("request record does not hold %q; log: %q", strings.TrimSpace(tc.want), out)
			}
			if strings.Contains(out, tc.forbidden) {
				t.Errorf("the log holds %q, more of the client address than its network; log: %q", tc.forbidden, out)
			}
		})
	}
}
