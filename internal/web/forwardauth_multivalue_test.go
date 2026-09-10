package web

import (
	"net/http/httptest"
	"testing"
)

// An identity header that arrives more than once is not an identity.
//
// Header.Get returns the first value under a key and silently drops the rest.
// A proxy that appends its own value instead of replacing the client's — the
// shape of CVE-2026-30851 on Caddy 2.10.0–2.11.1 when the outpost answers 200
// without that header — leaves two values under one canonical key, and which
// one Get picks is decided by the order they were added in. With the client's
// value first that is a complete takeover: the username, or a groups header
// carrying the administration group.
//
// The strip runs after the read and cannot help. Every spelling test in this
// package sets its headers with Header.Set, which replaces, so none of them
// could see this; the audit of Phase 10 measured it with a probe.
func TestForwardAuthBelievesNoIdentityHeaderThatArrivesTwice(t *testing.T) {
	for _, name := range []string{
		"X-Authentik-Username",
		"X-Authentik-Email",
		"X-Authentik-Name",
		"X-Authentik-Groups",
	} {
		for _, order := range [][2]string{{"stranger", "genuine"}, {"genuine", "stranger"}} {
			t.Run(name+"/"+order[0]+"-first", func(t *testing.T) {
				req := httptest.NewRequest("GET", "/admin/", nil)
				req.RemoteAddr = "127.0.0.1:41234"
				req.Header.Set(ProxySecretHeader, testProxySecret)
				req.Header.Set("X-authentik-username", "ada")
				req.Header.Set("X-authentik-email", "ada@example.com")
				req.Header.Set("X-authentik-name", "Ada Lovelace")
				req.Header.Set("X-authentik-groups", "redaktion-a")
				values := []string{order[0], order[1]}
				req.Header[name] = values

				p := serveForwardAuth(t, req, ForwardAuthOptions{Enabled: true, Secret: testProxySecret})

				if p.found {
					// values, not req.Header[name]: the strip has emptied that by now.
					t.Errorf("%s arrived with two values %q and an identity was believed (%+v); "+
						"which value Header.Get returns is decided by whoever added theirs first",
						name, values, p.identity)
				}
				assertNoIdentityHeaderSurvives(t, p.header)
			})
		}
	}

	// The control: the same request with every header carrying one value is
	// believed, so the refusals above are about multiplicity and nothing else.
	req := httptest.NewRequest("GET", "/admin/", nil)
	req.RemoteAddr = "127.0.0.1:41234"
	req.Header.Set(ProxySecretHeader, testProxySecret)
	req.Header.Set("X-authentik-username", "ada")
	req.Header.Set("X-authentik-email", "ada@example.com")
	req.Header.Set("X-authentik-name", "Ada Lovelace")
	req.Header.Set("X-authentik-groups", "redaktion-a")
	if p := serveForwardAuth(t, req, ForwardAuthOptions{Enabled: true, Secret: testProxySecret}); !p.found {
		t.Fatal("the control request with one value per header was not believed; the " +
			"assertions above would be measuring something other than multiplicity")
	}
}
