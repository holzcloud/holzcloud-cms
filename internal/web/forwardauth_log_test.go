package web

import (
	"bytes"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var logged bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &logged
}

// A trusted peer that sends identity headers with a wrong secret leaves a line
// in the log.
//
// Constant time keeps the comparison from leaking timing; it does nothing
// against a process that counts as a trusted peer and simply guesses. A wrong
// secret used to be ignored without a trace, so a guessing loop left nothing
// behind (Phase 10 code review WR-06). The line names neither the secret that
// arrived nor the one configured.
func TestForwardAuthLogsAWrongSecretFromATrustedPeer(t *testing.T) {
	logged := captureLog(t)
	const guess = "not-the-secret-but-a-guess-of-some-length"

	req := httptest.NewRequest("GET", "/admin/", nil)
	req.RemoteAddr = "127.0.0.1:41234"
	req.Header.Set("X-authentik-username", "ada")
	req.Header.Set(ProxySecretHeader, guess)
	p := serveForwardAuth(t, req, ForwardAuthOptions{Enabled: true, Secret: testProxySecret})
	if p.found {
		t.Fatal("a wrong secret was believed")
	}

	out := logged.String()
	if !strings.Contains(out, "wrong shared secret") {
		t.Errorf("nothing was logged for identity headers with a wrong secret from a trusted peer; log: %q", out)
	}
	for _, secret := range []string{guess, testProxySecret} {
		if strings.Contains(out, secret) {
			t.Errorf("the log carries a secret: %q", out)
		}
	}
}

// Nothing is logged where there is nothing to say: an untrusted peer, whose
// headers are never read at all, and a trusted peer sending no identity.
func TestForwardAuthLogsNothingForAnUntrustedPeerOrNoIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, remote string
		identity     bool
	}{
		{"an untrusted peer with identity headers and a wrong secret", "192.0.2.7:41234", true},
		{"a trusted peer with no identity header", "127.0.0.1:41234", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logged := captureLog(t)
			req := httptest.NewRequest("GET", "/admin/", nil)
			req.RemoteAddr = tc.remote
			if tc.identity {
				req.Header.Set("X-authentik-username", "ada")
				req.Header.Set(ProxySecretHeader, "not-the-secret-but-a-guess-of-some-length")
			}
			serveForwardAuth(t, req, ForwardAuthOptions{Enabled: true, Secret: testProxySecret})
			if strings.Contains(logged.String(), "wrong shared secret") {
				t.Errorf("logged a wrong secret where there was none to log: %q", logged.String())
			}
		})
	}
}
