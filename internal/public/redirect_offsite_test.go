package public

import (
	"context"
	"net/http"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/page"
)

// A redirect row written by an earlier version may name another server in a
// form a browser reads as one (//host, /\host, a tab inside the path). The
// visitor gets the 404 page instead, in the main language and under a prefix.
func TestOffsiteRedirectIsNotFollowed(t *testing.T) {
	h, database := newTestHandler(t)
	ws := multilingualSite(t, database, "fr")
	de := seedPageIn(t, database, ws.ID, "Kontakt", "kontakt", "Ruf an", "published", "", 0)
	seedPageIn(t, database, ws.ID, "Contact", "kontakt", "Appelez", "published", "fr", de.ID)

	store := page.NewStore(database)
	ctx := context.Background()
	for _, row := range []struct{ from, to string }{
		{"/a", "//evil.example/x"},
		{"/b", `/\evil.example`},
		{"/c", "/\t/evil.example"},
		{"/d", "/kontakt"},
		{"/e", "https://example.com/ok"},
	} {
		if err := store.AddRedirect(ctx, ws.ID, row.from, row.to, 301); err != nil {
			t.Fatalf("AddRedirect(%q): %v", row.from, err)
		}
	}

	for _, from := range []string{"/a", "/b", "/c", "/fr/a", "/fr/b"} {
		rec := throughMiddleware(h.HandlePage, ws, from, "slug")
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s answers %d; want 404", from, rec.Code)
		}
		if loc := rec.Header().Get("Location"); loc != "" {
			t.Errorf("%s sends Location %q; want none", from, loc)
		}
	}

	for _, c := range []struct{ from, want string }{
		{"/d", "/kontakt"},
		{"/fr/d", "/fr/kontakt"},
		{"/e", "https://example.com/ok"},
	} {
		rec := throughMiddleware(h.HandlePage, ws, c.from, "slug")
		if rec.Code != http.StatusMovedPermanently {
			t.Errorf("%s answers %d; want 301", c.from, rec.Code)
			continue
		}
		if got := rec.Header().Get("Location"); got != c.want {
			t.Errorf("%s redirects to %q; want %q", c.from, got, c.want)
		}
	}
}
