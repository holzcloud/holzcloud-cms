package public

import (
	"net/http"
	"net/netip"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// behindAProxy is a handler whose resolver believes X-Forwarded-For from
// 127.0.0.1 and from nothing else, the shape of a reverse proxy on the same
// machine.
func behindAProxy(t *testing.T) (*Handler, *domain.Website) {
	t.Helper()
	h, ws := boundsCase(t, 5<<20, 50<<20)
	h.SetClientIP(web.NewClientIPResolver([]netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}))
	return h, ws
}

// via makes a request look as if the proxy at 127.0.0.1 forwarded it for ip.
func via(ip string) func(*http.Request) {
	return func(r *http.Request) {
		r.RemoteAddr = "127.0.0.1:4711"
		r.Header.Set("X-Forwarded-For", ip)
	}
}

// from makes a request arrive directly from a peer address.
func from(peer string) func(*http.Request) {
	return func(r *http.Request) { r.RemoteAddr = peer }
}

// submitFiles takes a small multipart submission from the request shape prepare
// makes. The keeper is returned open; the caller closes it or leaves it
// inside.
func submitFiles(t *testing.T, h *Handler, ws *domain.Website, prepare func(*http.Request)) (*attachmentKeeper, *uploadRefusal) {
	t.Helper()
	body, ctype := rawMultipart(t, false, fld("name", "Anna"))
	_, keeper, took, refused := takeBody(t, h, ws, body, ctype, prepare)
	if took == (refused != nil) {
		t.Fatalf("took=%v refused=%+v: one of the two, not both", took, refused)
	}
	return keeper, refused
}

// One network holds at most two of the four slots, by its IPv4 address or its
// IPv6 /64.
func TestOneNetworkHoldsAtMostTwoUploadsAtOnce(t *testing.T) {
	h, ws := behindAProxy(t)

	for i, ip := range []string{"2001:db8:1:2::1", "2001:db8:1:2:aaaa::7"} {
		if _, refused := submitFiles(t, h, ws, via(ip)); refused != nil {
			t.Fatalf("submission %d of the network was refused: %+v", i+1, refused)
		}
	}
	_, refused := submitFiles(t, h, ws, via("2001:db8:1:2::ffff"))
	if refused == nil || refused.Status != http.StatusTooManyRequests {
		t.Fatalf("a third from the same /64: refused=%+v, want 429", refused)
	}
	if refused.RetryAfter < time.Second {
		t.Errorf("Retry-After %v, want at least one second", refused.RetryAfter)
	}

	// Another /64 and an IPv4 address are other networks, and are taken.
	for _, ip := range []string{"2001:db8:1:3::1", "203.0.113.7"} {
		if _, refused := submitFiles(t, h, ws, via(ip)); refused != nil {
			t.Errorf("%s was refused although its network holds none: %+v", ip, refused)
		}
	}
	// The server is full now (four inside): the next stranger is told so, with
	// the other status.
	_, refused = submitFiles(t, h, ws, via("198.51.100.9"))
	if refused == nil || refused.Status != http.StatusServiceUnavailable {
		t.Errorf("a fifth submission: refused=%+v, want 503", refused)
	}
}

// A network sends at most thirty submissions with files an hour. The server
// being full is not the network's doing and does not count.
func TestANetworkSendsAtMostThirtyUploadsAnHour(t *testing.T) {
	h, ws := behindAProxy(t)

	for i := 1; i <= uploadNetworkPerHour; i++ {
		keeper, refused := submitFiles(t, h, ws, via("203.0.113.30"))
		if refused != nil {
			t.Fatalf("submission %d of %d was refused: %+v", i, uploadNetworkPerHour, refused)
		}
		keeper.Close()
	}
	_, refused := submitFiles(t, h, ws, via("203.0.113.30"))
	if refused == nil || refused.Status != http.StatusTooManyRequests {
		t.Fatalf("the thirty-first: refused=%+v, want 429", refused)
	}
	if refused.RetryAfter <= time.Minute {
		t.Errorf("Retry-After %v, want the wait of the window (well over a minute)", refused.RetryAfter)
	}
	if keeper, refused := submitFiles(t, h, ws, via("203.0.113.31")); refused != nil {
		t.Errorf("another network was refused: %+v", refused)
	} else {
		keeper.Close()
	}

	// Refused for a full server: nothing is counted against the network.
	leaves := fillTheServer(t, h.uploads)
	for i := 0; i < 10; i++ {
		_, refused := submitFiles(t, h, ws, via("203.0.113.32"))
		if refused == nil || refused.Status != http.StatusServiceUnavailable {
			t.Fatalf("refusal %d with the server full: %+v, want 503", i+1, refused)
		}
	}
	for _, leave := range leaves {
		leave()
	}
	for i := 1; i <= uploadNetworkPerHour; i++ {
		keeper, refused := submitFiles(t, h, ws, via("203.0.113.32"))
		if refused != nil {
			t.Fatalf("submission %d after ten refusals for a full server: %+v: they were counted", i, refused)
		}
		keeper.Close()
	}
}

// An address that names no visitor is every visitor behind a proxy that was
// not told about: it meets the server's bound and no network's, or one
// stranger would close the form for all.
func TestAnAddressThatNamesNoVisitorMeetsOnlyTheServerBound(t *testing.T) {
	for name, peer := range map[string]string{
		"an untrusted private peer": "10.0.0.5:5555",
		"a loopback peer":           "127.0.0.1:5555",
	} {
		t.Run(name, func(t *testing.T) {
			h, ws := behindAProxy(t)

			var open []*attachmentKeeper
			for i := 1; i <= uploadSlots; i++ {
				keeper, refused := submitFiles(t, h, ws, from(peer))
				if refused != nil {
					t.Fatalf("submission %d at once was refused: %+v: a network bound applied", i, refused)
				}
				open = append(open, keeper)
			}
			_, refused := submitFiles(t, h, ws, from(peer))
			if refused == nil || refused.Status != http.StatusServiceUnavailable {
				t.Fatalf("the fifth: refused=%+v, want 503 (the server bound)", refused)
			}
			for _, keeper := range open {
				keeper.Close()
			}

			for i := 1; i <= 40; i++ {
				keeper, refused := submitFiles(t, h, ws, from(peer))
				if refused != nil {
					t.Fatalf("submission %d in a row was refused: %+v: an hourly budget applied", i, refused)
				}
				keeper.Close()
			}
		})
	}
}
