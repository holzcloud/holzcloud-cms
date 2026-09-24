package export

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// site is a small website answered by hand, so each rule of the export can be
// seen on its own.
func site() http.Handler {
	mux := http.NewServeMux()
	page := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(body))
		}
	}
	mux.HandleFunc("GET /{$}", page(`<link rel="stylesheet" href="/t/style.css?v=abc">
<a href="/blog">Blog</a> <a href="/alt">Alt</a> <a href="/suche?q=holz">Suche</a>
<a href="/warenkorb">Warenkorb</a> <a href="https://example.org/">weg</a>
<a href="https://demo.test/absolut">absolut</a> <img srcset="/media/1/a.jpg 1x, /media/1/b.jpg 2x">`))
	mux.HandleFunc("GET /blog", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Query().Get("seite") == "2" {
			w.Write([]byte(`zweite <a href="/blog?seite=1">zurück</a>`))
			return
		}
		w.Write([]byte(`erste <a href="/blog?seite=2">weiter</a> <form method="post" action="/kontakt"></form>`))
	})
	mux.HandleFunc("GET /alt", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/blog", http.StatusMovedPermanently)
	})
	mux.HandleFunc("GET /absolut", page("absolut"))
	mux.HandleFunc("GET /t/style.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		w.Write([]byte(`body{background:url("/t/bg.png")}`))
	})
	mux.HandleFunc("GET /t/bg.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("png"))
	})
	mux.HandleFunc("GET /media/1/{f}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write([]byte("jpg"))
	})
	return mux
}

func runSite(t *testing.T) (string, *Report) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "out")
	r, err := Run(context.Background(), Options{Handler: site(), Host: "demo.test", Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	return dir, r
}

func read(t *testing.T, dir, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	return string(b)
}

func TestEveryLinkedFileIsWritten(t *testing.T) {
	dir, r := runSite(t)
	for _, f := range []string{"index.html", "blog/index.html", "t/style.css", "t/bg.png", "media/1/a.jpg", "media/1/b.jpg", "absolut/index.html"} {
		read(t, dir, f)
	}
	for _, f := range r.Files {
		if strings.Contains(f, "example.org") || strings.HasPrefix(f, "warenkorb") {
			t.Errorf("%s should not be in the export", f)
		}
	}
}

// A static host does not look at the query, so the second page of a list must
// become a path of its own, and the link to it must say so.
func TestAListsSecondPageBecomesAPath(t *testing.T) {
	dir, _ := runSite(t)
	if got := read(t, dir, "blog/seite/2/index.html"); !strings.Contains(got, "zweite") {
		t.Errorf("second page: %q", got)
	}
	first := read(t, dir, "blog/index.html")
	if !strings.Contains(first, `href="/blog/seite/2/"`) {
		t.Errorf("the link to page two was not rewritten: %s", first)
	}
	if second := read(t, dir, "blog/seite/2/index.html"); !strings.Contains(second, `href="/blog"`) {
		t.Errorf("the link back to page one was not rewritten: %s", second)
	}
}

func TestWhatCannotBeStaticIsReported(t *testing.T) {
	_, r := runSite(t)
	got := map[string]string{}
	for _, s := range r.Skipped {
		got[s.URL] = s.Reason
	}
	if got["/suche?q=holz"] != ReasonQuery {
		t.Errorf("search: %q", got["/suche?q=holz"])
	}
	if got["/warenkorb"] != ReasonServer {
		t.Errorf("cart: %q", got["/warenkorb"])
	}
	if len(r.Forms) != 1 || r.Forms[0] != "/blog" {
		t.Errorf("forms: %v", r.Forms)
	}
}

func TestARedirectBecomesARefreshPage(t *testing.T) {
	dir, _ := runSite(t)
	got := read(t, dir, "alt/index.html")
	if !strings.Contains(got, `http-equiv="refresh" content="0; url=/blog"`) {
		t.Errorf("redirect page: %s", got)
	}
}

func TestFilePathStaysInsideTheDirectory(t *testing.T) {
	for _, p := range []string{"/../../etc/passwd", "/a/../../b"} {
		u := mustParse(t, p)
		if got := filePath(u, false); strings.HasPrefix(got, "..") || strings.HasPrefix(got, "/") {
			t.Errorf("%s -> %s", p, got)
		}
	}
}

func mustParse(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}
