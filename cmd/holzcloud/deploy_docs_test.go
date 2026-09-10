package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests read the deployment documents an operator copies from. Until the
// Phase 10 audit nothing did: the delete lines in the Caddyfile and the minimum
// Caddy version in DEPLOY.md were checked once, by hand, when the plan was
// executed, and could have disappeared the next day without a test noticing
// (threat T-10-44). Against a Caddy that forwards a visitor's own identity header
// under the canonical name, those lines are the only defence there is.

func readDeploy(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "deploy", name))
	if err != nil {
		t.Fatalf("read deploy/%s: %v", name, err)
	}
	return string(b)
}

// caddyLines returns the example's lines with the comment markers taken off,
// because the shipped example carries the forward-auth block commented out.
func caddyLines(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, line := range strings.Split(readDeploy(t, "Caddyfile.example"), "\n") {
		out = append(out, strings.TrimLeft(line, "#\t "))
	}
	return out
}

func TestCaddyfileDeletesEveryCopiedHeaderInBothSpellings(t *testing.T) {
	lines := caddyLines(t)
	copyAt, forwardAt := -1, -1
	var copied []string
	for i, l := range lines {
		if strings.HasPrefix(l, "copy_headers ") {
			copyAt = i
			copied = strings.Fields(strings.TrimPrefix(l, "copy_headers "))
		}
		if strings.HasPrefix(l, "forward_auth ") {
			forwardAt = i
		}
	}
	if copyAt < 0 || forwardAt < 0 || len(copied) == 0 {
		t.Fatal("deploy/Caddyfile.example has no forward_auth block with copy_headers")
	}
	for _, header := range copied {
		for _, spelling := range []string{header, strings.ReplaceAll(header, "-", "_")} {
			want := "request_header -" + spelling
			at := -1
			for i, l := range lines {
				if l == want {
					at = i
				}
			}
			switch {
			case at < 0:
				t.Errorf("deploy/Caddyfile.example copies %s but has no %q", header, want)
			case at > forwardAt:
				t.Errorf("%q stands after forward_auth; the visitor's copy must be gone before the outpost is asked", want)
			}
		}
	}
}

func TestCaddyfilePassesTheSecretFromTheEnvironment(t *testing.T) {
	found := false
	for _, l := range caddyLines(t) {
		if strings.HasPrefix(l, "header_up X-Holzcloud-Proxy-Secret") {
			found = true
			if got := strings.TrimSpace(strings.TrimPrefix(l, "header_up X-Holzcloud-Proxy-Secret")); got != "{env.HOLZCLOUD_SSO_SECRET}" {
				t.Errorf("the proxy secret is set to %q; a Caddyfile is a file people paste into forums, the value belongs in the environment", got)
			}
		}
	}
	if !found {
		t.Error("deploy/Caddyfile.example never sends X-Holzcloud-Proxy-Secret")
	}
}

func TestDeployNamesTheMinimumCaddy(t *testing.T) {
	doc := readDeploy(t, "DEPLOY.md")
	for _, want := range []string{"2.11.2", "CVE-2026-30851"} {
		if !strings.Contains(doc, want) {
			t.Errorf("deploy/DEPLOY.md does not name %s", want)
		}
	}
}

// TestDeployTellsHowTheSecretReachesCaddy: the Caddyfile reads the secret from
// Caddy's environment, and nothing said how it gets there, while the service
// unit asks for payment and mail secrets in a 0600 environment file
// (threat T-10-45).
func TestDeployTellsHowTheSecretReachesCaddy(t *testing.T) {
	doc := readDeploy(t, "DEPLOY.md")
	for _, want := range []string{"systemctl edit caddy", "EnvironmentFile=/etc/holzcloud-sso.env"} {
		if !strings.Contains(doc, want) {
			t.Errorf("deploy/DEPLOY.md does not say how HOLZCLOUD_SSO_SECRET reaches Caddy (missing %q)", want)
		}
	}
}

// TestNoDeployDocumentCallsAForwardingProxyHarmless: the stripping inside the
// CMS runs after the identity headers are read, so it does not turn a proxy
// that forwards a visitor's value into a mere misconfiguration. Three documents
// said it did until the Phase 10 code review (WR-03); this holds the one that
// is copied most.
func TestNoDeployDocumentCallsAForwardingProxyHarmless(t *testing.T) {
	for _, name := range []string{"Caddyfile.example", "DEPLOY.md"} {
		doc := readDeploy(t, name)
		for _, claim := range []string{"not a way in", "rather than a way in", "rather than a bypass"} {
			if strings.Contains(doc, claim) {
				t.Errorf("deploy/%s still says %q about the strip", name, claim)
			}
		}
	}
}
