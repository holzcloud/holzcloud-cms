package public

import (
	"errors"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/media"
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

// submitFiles takes a small multipart submission with one file from the
// request shape prepare makes. The keeper is returned open; the caller closes
// it or leaves it inside.
func submitFiles(t *testing.T, h *Handler, ws *domain.Website, prepare func(*http.Request)) (*attachmentKeeper, *uploadRefusal) {
	t.Helper()
	body, ctype := rawMultipart(t, false, fld("name", "Anna"), filePart("anhang", "bild.png", bigPNG(1024)))
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

// Below the free-space floor a file is refused with the catalogue's sentence,
// in the page's language, and nothing is spooled for it; the fields are still
// handed on, so a message without a file arrives.
func TestBelowTheFreeSpaceFloorFilesAreRefusedAndTheMessageStillArrives(t *testing.T) {
	h, ws := boundsCase(t, 5<<20, 50<<20)
	h.SetMinFreeBytes(1 << 30)
	floor := h.AttachmentFloor()
	// Each of the four places may hold its largest submission twice at once:
	// in the spool and, while it is kept, as the stored copy beside it
	// (WR-05 of the phase 38 review).
	if want := uint64(1<<30) + uploadSlots*2*uint64(attachmentBound(media.Limits{Image: 5 << 20, Video: 50 << 20})); floor != want {
		t.Fatalf("AttachmentFloor = %d, want the floor plus twice four of the largest submissions (%d)", floor, want)
	}
	body, ctype := rawMultipart(t, false, fld("name", "Anna"), filePart("anhang", "bild.png", onePixel(t)))
	french := func(r *http.Request) { *r = *r.WithContext(i18n.WithLang(r.Context(), "fr")) }

	for name, seam := range map[string]func() (uint64, error){
		"one byte short of the floor": func() (uint64, error) { return floor - 1, nil },
		"no space at all":             func() (uint64, error) { return 0, nil },
		"the free space is unreadable": func() (uint64, error) {
			return 1 << 62, errors.New("statfs: the filesystem is gone")
		},
	} {
		t.Run(name, func(t *testing.T) {
			h.freeBytes = seam
			fields, keeper, took, refused := takeBody(t, h, ws, body, ctype, french)
			if !took || refused != nil {
				t.Fatalf("took=%v refused=%+v: a low disk must refuse the file, not the message", took, refused)
			}
			if fields != "name=Anna" {
				t.Errorf("fields = %q, want the message handed on", fields)
			}
			listed := keeper.List()
			want := i18n.T("fr", "The file could not be accepted.")
			if want == "The file could not be accepted." {
				t.Fatalf("the sentence has no French translation: %q", want)
			}
			if len(listed) != 1 || listed[0].Refused != want || listed[0].MimeType != "" {
				t.Errorf("listed = %+v, want the file named and refused with %q", listed, want)
			}
			if names := spoolEntries(t, h.dataDir); len(names) != 0 {
				t.Errorf("the spool holds %v: a file was written below the floor", names)
			}
		})
	}

	// Exactly the floor, and plenty: the file is taken.
	for _, free := range []uint64{floor, floor + 1<<30} {
		h.freeBytes = func() (uint64, error) { return free, nil }
		_, keeper, took, refused := takeBody(t, h, ws, body, ctype)
		if !took || refused != nil {
			t.Fatalf("with %d free: took=%v refused=%+v", free, took, refused)
		}
		if listed := keeper.List(); len(listed) != 1 || listed[0].Refused != "" || len(spoolFiles(t, h.dataDir)) != 1 {
			t.Errorf("with %d free: listed = %+v, want the file taken", free, listed)
		}
		keeper.Close()
	}
}

// The same through the real contact form: a message with a file is answered as
// a refused file, a message without one is stored.
func TestLowDiskThroughTheContactForm(t *testing.T) {
	h, database, ws, _ := formularAufbau(t)
	h.SetUploadLimits(5<<20, 50<<20)
	h.SetMinFreeBytes(1 << 30)
	h.freeBytes = func() (uint64, error) { return 1 << 20, nil }

	form := func(extra ...formPart) ([]byte, string) {
		parts := []formPart{
			fld("seite", "kontakt"), fld("pfad", "/kontakt"),
			fld("gestellt", timeToken(t, database, -10*time.Second)),
			fld("name", "Eva Muster"), fld("email", "eva@example.test"),
			fld("nachricht", "Guten Tag."),
		}
		return rawMultipart(t, false, append(parts, extra...)...)
	}

	body, ctype := form(filePart("anhang", "bild.png", onePixel(t)))
	rec := postMultipart(t, h, ws, "/formular", body, ctype)
	if ort := rec.Header().Get("Location"); rec.Code != http.StatusSeeOther || !strings.Contains(ort, "formular=fehler") {
		t.Fatalf("a file on a low disk: %d %q, want 303 formular=fehler", rec.Code, ort)
	}
	var media int
	if err := database.Read.QueryRow(`SELECT COUNT(*) FROM media WHERE owner_plugin = 'kontaktformular'`).Scan(&media); err != nil || media != 0 {
		t.Errorf("%d media rows after a refused file, %v; want none", media, err)
	}
	if n := messages(t, database); n != 0 {
		t.Errorf("%d messages stored for a message whose file was refused, want none", n)
	}

	body, ctype = form()
	rec = postMultipart(t, h, ws, "/formular", body, ctype)
	if ort := rec.Header().Get("Location"); rec.Code != http.StatusSeeOther || !strings.Contains(ort, "formular=gesendet") {
		t.Fatalf("a message without a file on a low disk: %d %q, want 303 formular=gesendet", rec.Code, ort)
	}
	if n := messages(t, database); n != 1 {
		t.Errorf("%d messages stored, want the one without a file", n)
	}
	spoolEmpty(t, h, "low disk")
}
