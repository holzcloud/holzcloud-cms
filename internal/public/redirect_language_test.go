package public

import (
	"context"
	"net/http"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// A rename writes one redirect for the bare address, because a slug is the
// same in every language. A visitor of the old address in a further language
// has to land on the new address in that language, not in the main one; and a
// redirect the operator wrote for the prefixed address wins over that.
func TestRedirectsKeepTheLanguage(t *testing.T) {
	h, database := newTestHandler(t)
	ws := multilingualSite(t, database, "fr,en")
	de := seedPageIn(t, database, ws.ID, "Neu", "neu", "Hallo", "published", "", 0)
	seedPageIn(t, database, ws.ID, "Nouveau", "neu", "Salut", "published", "fr", de.ID)
	seedPageIn(t, database, ws.ID, "New", "neu", "Hello", "published", "en", de.ID)

	store := page.NewStore(database)
	ctx := context.Background()
	if err := store.AddRedirect(ctx, ws.ID, "/alt", "/neu", 301); err != nil {
		t.Fatalf("AddRedirect: %v", err)
	}
	if err := store.AddRedirect(ctx, ws.ID, "/en/alt", "/en/neu#hier", 301); err != nil {
		t.Fatalf("AddRedirect: %v", err)
	}
	if err := store.AddRedirect(ctx, ws.ID, "/weg", "https://example.com/weg", 301); err != nil {
		t.Fatalf("AddRedirect: %v", err)
	}

	for _, c := range []struct{ from, want string }{
		{"/alt", "/neu"},
		{"/fr/alt", "/fr/neu"},
		{"/en/alt", "/en/neu#hier"},
		{"/fr/weg", "https://example.com/weg"},
	} {
		rec := throughMiddleware(h.HandlePage, ws, c.from, "slug")
		if rec.Code != http.StatusMovedPermanently {
			t.Errorf("%s answers %d instead of 301", c.from, rec.Code)
			continue
		}
		if got := rec.Header().Get("Location"); got != c.want {
			t.Errorf("%s redirects to %q instead of %q", c.from, got, c.want)
		}
	}
}

func TestWithLanguagePrefix(t *testing.T) {
	for _, c := range []struct{ tag, to, want string }{
		{"", "/neu", "/neu"},
		{"fr", "/neu", "/fr/neu"},
		{"fr", "/", "/fr"},
		{"fr", "/fr/neu", "/fr/neu"},
		{"fr", "/fr", "/fr"},
		{"fr", "/frage", "/fr/frage"},
		{"fr", "https://example.com/x", "https://example.com/x"},
		{"fr", "//example.com/x", "//example.com/x"},
	} {
		if got := withLanguagePrefix(c.tag, c.to); got != c.want {
			t.Errorf("withLanguagePrefix(%q, %q) = %q, want %q", c.tag, c.to, got, c.want)
		}
	}
}
