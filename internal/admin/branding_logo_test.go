package admin

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/branding"
)

// brandingDir points the branding package at a folder of this test's own, so
// two tests never share a logo and none of them writes into the repository.
func brandingDir(t *testing.T) string {
	t.Helper()
	folder := t.TempDir()
	branding.SetDir(folder)
	t.Cleanup(func() { branding.SetDir("") })
	return folder
}

// A valid one-pixel PNG, so a test that wants an accepted upload has one.
var onePixelPNG = []byte{
	0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n',
	0, 0, 0, 0x0d, 'I', 'H', 'D', 'R',
	0, 0, 0, 1, 0, 0, 0, 1, 8, 6, 0, 0, 0, 0x1f, 0x15, 0xc4, 0x89,
	0, 0, 0, 0x0a, 'I', 'D', 'A', 'T', 0x78, 0x9c, 0x63, 0, 1, 0, 0, 5, 0, 1,
	0x0d, 0x0a, 0x2d, 0xb4,
	0, 0, 0, 0, 'I', 'E', 'N', 'D', 0xae, 0x42, 0x60, 0x82,
}

// The logo screen, which had no test of any kind.
//
// Driven red the way TEST-03 asks: sixteen mutations of the check below, of
// which fifteen were caught. The one that was not is equivalent — `depth == 0`
// in the final return is unreachable as a distinguisher, because a truncated
// document produces a decoder error first. It is recorded in branding.go at the
// line rather than removed.
//
// The tests came first here and they earned it: six of them were red against
// the check as it stood, and that is how the gap in it was found at all.
//
// looksLikeSVG is the first of two belts. The second is the content security
// policy, which forbids inline script whatever this function says — so nothing
// here is an open door. What is at stake is the claim the function makes about
// itself: "carries nothing executable". A check that says that and does not do
// it is the kind of comment phase 17 spent a plan removing.
func TestAnSVGThatCarriesScriptIsRefused(t *testing.T) {
	for _, c := range []struct {
		what string
		doc  string
	}{
		{"a script element", `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`},
		{"an onload attribute", `<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"/>`},
		// The four below are the same thing spelled differently, and a
		// substring denylist of four strings let every one of them through.
		{"an onerror attribute", `<svg xmlns="http://www.w3.org/2000/svg" onerror="alert(1)"><circle r="1"/></svg>`},
		{"an onmouseover attribute", `<svg xmlns="http://www.w3.org/2000/svg" onmouseover="alert(1)"/>`},
		{"a space before the equals sign", `<svg xmlns="http://www.w3.org/2000/svg" onload ="alert(1)"/>`},
		{"a capital letter in the attribute", `<svg xmlns="http://www.w3.org/2000/svg" onLoad="alert(1)"/>`},
		{"a javascript: link", `<svg xmlns="http://www.w3.org/2000/svg"><a href="javascript:alert(1)">x</a></svg>`},
		{"a javascript: link written as an entity", `<svg xmlns="http://www.w3.org/2000/svg"><a href="javascript&#58;alert(1)">x</a></svg>`},
		{"a foreignObject, which is HTML in disguise", `<svg xmlns="http://www.w3.org/2000/svg"><foreignObject><body xmlns="http://www.w3.org/1999/xhtml">x</body></foreignObject></svg>`},
		{"a handler element, which is SVG Tiny's own script", `<svg xmlns="http://www.w3.org/2000/svg"><handler type="text/ecmascript">x</handler></svg>`},
		{"a javascript: link behind whitespace, which a browser strips",
			"<svg xmlns=\"http://www.w3.org/2000/svg\"><a href=\"  javascript:alert(1)\">x</a></svg>"},
		{"a JavaScript: link in capitals",
			`<svg xmlns="http://www.w3.org/2000/svg"><a href="JavaScript:alert(1)">x</a></svg>`},
		{"a vbscript: link, for the browsers that still know it",
			`<svg xmlns="http://www.w3.org/2000/svg"><a href="vbscript:msgbox(1)">x</a></svg>`},
		// Without the angle brackets an HTML payload would name in the test,
		// because a `<` inside an attribute value is a parse error and the
		// document would then be refused for a reason that has nothing to do
		// with the scheme.
		{"a data: URL carrying HTML",
			`<svg xmlns="http://www.w3.org/2000/svg"><a href="data:text/html,hallo">x</a></svg>`},
		// XML is case-sensitive, so ONLOAD is not an event handler to a parser
		// that knows it is reading XML. Refused all the same: the cost is an
		// attribute nobody writes, and the alternative is trusting every
		// browser to have decided the same thing.
		{"an event attribute shouted", `<svg xmlns="http://www.w3.org/2000/svg" ONERROR="alert(1)"/>`},
		{"an entity declaration, which is the billion-laughs shape",
			"<?xml version=\"1.0\"?><!DOCTYPE svg [<!ENTITY a \"aaaaaaaaaa\">]><svg xmlns=\"http://www.w3.org/2000/svg\"><title>x</title></svg>"},
	} {
		if looksLikeSVG([]byte(c.doc)) {
			t.Errorf("%s was accepted: %s", c.what, c.doc)
		}
	}
}

// And the other direction, which matters just as much: a refusal that refuses
// everything is not a check, it is a broken upload button. These are the shapes
// a drawing program actually writes.
func TestAnOrdinarySVGIsAccepted(t *testing.T) {
	for _, c := range []struct{ what, doc string }{
		{"the plainest one there is",
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><circle cx="5" cy="5" r="4"/></svg>`},
		{"with an XML declaration in front",
			"<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<svg xmlns=\"http://www.w3.org/2000/svg\"><path d=\"M0 0 L10 10\"/></svg>"},
		{"with leading whitespace",
			"\n  <svg xmlns=\"http://www.w3.org/2000/svg\"><rect width=\"4\" height=\"4\"/></svg>\n"},
		{"with a comment and a title",
			`<svg xmlns="http://www.w3.org/2000/svg"><!-- gezeichnet --><title>Logo</title><g fill="#8b5a2b"><rect width="4" height="4"/></g></svg>`},
		{"with the namespace prefixes Inkscape writes",
			`<svg xmlns="http://www.w3.org/2000/svg" xmlns:svg="http://www.w3.org/2000/svg" xmlns:inkscape="http://www.inkscape.org/namespaces/inkscape" inkscape:version="1.1" version="1.1"><path inkscape:connector-curvature="0" d="M0 0"/></svg>`},
		{"with a style element, which is not script",
			`<svg xmlns="http://www.w3.org/2000/svg"><style>.a{fill:#000}</style><rect class="a" width="4" height="4"/></svg>`},
		// Text is text: a word that happens to begin with "on" is not an
		// attribute, and an operator whose logo says so must still be able to
		// upload it.
		{"with text that reads like an attribute",
			`<svg xmlns="http://www.w3.org/2000/svg"><text>one=two, on sale</text></svg>`},
	} {
		if !looksLikeSVG([]byte(c.doc)) {
			t.Errorf("%s was refused: %s", c.what, c.doc)
		}
	}
}

// What is not an SVG at all, however it is labelled.
func TestWhatIsNotAnSVGIsRefused(t *testing.T) {
	for _, c := range []struct{ what, doc string }{
		{"nothing", ""},
		{"only whitespace", "   \n\t"},
		{"a PNG", string(onePixelPNG)},
		{"HTML", `<!doctype html><html><body>x</body></html>`},
		{"XML that is not an SVG", `<?xml version="1.0"?><rss version="2.0"><channel/></rss>`},
		{"an SVG that is not closed", `<svg xmlns="http://www.w3.org/2000/svg"><circle r="1">`},
		{"plain text that starts with the word svg", "svg ist ein bildformat"},
		{"two root elements", `<svg xmlns="http://www.w3.org/2000/svg"/><svg xmlns="http://www.w3.org/2000/svg"/>`},
		// A whole SVG and then something that does not parse. The document is
		// complete by the time the rubbish starts, so every check that looks
		// only at the tree is already satisfied — it is the parse error that
		// has to count.
		{"a complete SVG with rubbish after it",
			`<svg xmlns="http://www.w3.org/2000/svg"><circle r="1"/></svg><<<>`},
	} {
		if looksLikeSVG([]byte(c.doc)) {
			t.Errorf("%s was accepted as an SVG: %q", c.what, c.doc)
		}
	}
}

// The screen, end to end: the file is checked on its bytes and not on its name.
func TestTheLogoScreenChecksTheBytesAndNotTheName(t *testing.T) {
	folder := brandingDir(t)
	h, sm, _, _ := newTestAdmin(t)

	for _, c := range []struct {
		what, filename string
		data           []byte
		wantStored     string
	}{
		{"a real PNG called .png", "logo.png", onePixelPNG, "logo.png"},
		{"a PNG called .svg", "logo.svg", onePixelPNG, ""},
		{"an SVG called .png", "logo.png", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), ""},
		{"an executable called .png", "logo.png", []byte("\x7fELF\x02\x01\x01"), ""},
		{"a real SVG called .svg", "logo.svg",
			[]byte(`<svg xmlns="http://www.w3.org/2000/svg"><circle r="1"/></svg>`), "logo.svg"},
		{"an SVG with a script in it", "logo.svg",
			[]byte(`<svg xmlns="http://www.w3.org/2000/svg" onerror="alert(1)"/>`), ""},
		{"an empty file", "logo.png", nil, ""},
		{"a file over the bound", "logo.png",
			append(append([]byte{}, onePixelPNG...), bytes.Repeat([]byte{0}, branding.MaxLogoBytes)...), ""},
	} {
		if err := branding.RemoveLogo(); err != nil {
			t.Fatalf("%s: RemoveLogo: %v", c.what, err)
		}

		req := logoRequest(t, c.filename, c.data)
		rec := serve(t, h, sm, h.HandleBranding, req)
		if rec.Code != http.StatusSeeOther {
			t.Errorf("%s: status %d, want 303", c.what, rec.Code)
		}

		got := ""
		if path := branding.LogoPath(); path != "" {
			got = filepath.Base(path)
		}
		if got != c.wantStored {
			t.Errorf("%s: stored %q, want %q", c.what, got, c.wantStored)
		}
		if c.wantStored != "" {
			stored, err := os.ReadFile(filepath.Join(folder, c.wantStored))
			if err != nil {
				t.Fatalf("%s: read back: %v", c.what, err)
			}
			if !bytes.Equal(stored, c.data) {
				t.Errorf("%s: the stored bytes are not the uploaded ones", c.what)
			}
		}
	}
}

// The name and the letter arrive through the same screen, and the picture is
// optional — a save without one keeps whatever is there.
func TestTheBrandScreenSavesTheNameAndKeepsThePicture(t *testing.T) {
	brandingDir(t)
	h, sm, database, _ := newTestAdmin(t)
	ctx := context.Background()

	rec := serve(t, h, sm, h.HandleBranding, logoRequest(t, "logo.png", onePixelPNG,
		"name", "Holzbau Schmidt", "zeichen", "HS"))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d, want 303", rec.Code)
	}
	branding.Load(ctx, database.Read)
	if got := branding.Current(); got.Name != "Holzbau Schmidt" || got.Mark != "HS" {
		t.Errorf("after the save: %+v", got)
	}
	if branding.LogoPath() == "" {
		t.Fatal("the picture was not stored")
	}

	// A second save with no file at all: the name changes, the picture stays.
	rec = serve(t, h, sm, h.HandleBranding, logoRequest(t, "", nil,
		"name", "Schmidt & Söhne", "zeichen", "S"))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("second save: status %d, want 303", rec.Code)
	}
	branding.Load(ctx, database.Read)
	if got := branding.Current(); got.Name != "Schmidt & Söhne" || got.Mark != "S" {
		t.Errorf("after the second save: %+v", got)
	}
	if branding.LogoPath() == "" {
		t.Error("a save without a file threw the picture away")
	}

	// And the remove button takes it away without touching the name.
	rec = serve(t, h, sm, h.HandleBranding, logoRequest(t, "", nil, "logo_entfernen", "1"))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("remove: status %d, want 303", rec.Code)
	}
	if branding.LogoPath() != "" {
		t.Error("the picture is still there")
	}
	branding.Load(ctx, database.Read)
	if got := branding.Current().Name; got != "Schmidt & Söhne" {
		t.Errorf("removing the picture changed the name to %q", got)
	}
}

// The screen itself renders, with the brand on it and the remove button only
// when there is something to remove.
func TestTheBrandScreenShowsWhatIsThere(t *testing.T) {
	brandingDir(t)
	h, sm, database, _ := newTestAdmin(t)
	ctx := context.Background()

	if err := branding.Save(ctx, database.Write, "Holzbau Schmidt", "HS"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	rec := serve(t, h, sm, h.HandleBranding, httptest.NewRequest(http.MethodGet, "/admin/marke", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Holzbau Schmidt") {
		t.Error("the screen does not show the name it would save")
	}

	// With a picture, the screen offers to remove it.
	if err := branding.WriteLogo(".png", onePixelPNG); err != nil {
		t.Fatalf("WriteLogo: %v", err)
	}
	rec = serve(t, h, sm, h.HandleBranding, httptest.NewRequest(http.MethodGet, "/admin/marke", nil))
	withLogo := rec.Body.String()
	if len(withLogo) <= len(body) {
		t.Error("the screen looks the same with a picture as without one")
	}
}

// The picture is served with the type its extension says, and cached hard —
// which is safe only because its address carries the file's time.
func TestTheLogoIsServedWithItsOwnType(t *testing.T) {
	brandingDir(t)
	h, sm, _, _ := newTestAdmin(t)

	rec := serve(t, h, sm, h.HandleBrandingLogo,
		httptest.NewRequest(http.MethodGet, "/admin/marke/logo", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("without a picture: status %d, want 404", rec.Code)
	}

	for _, c := range []struct{ ext, want string }{
		{".png", "image/png"},
		{".svg", "image/svg+xml"},
		{".webp", "image/webp"},
	} {
		if err := branding.WriteLogo(c.ext, []byte("egal")); err != nil {
			t.Fatalf("WriteLogo %s: %v", c.ext, err)
		}
		rec := serve(t, h, sm, h.HandleBrandingLogo,
			httptest.NewRequest(http.MethodGet, "/admin/marke/logo", nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", c.ext, rec.Code)
			continue
		}
		if got := rec.Header().Get("Content-Type"); got != c.want {
			t.Errorf("%s: Content-Type %q, want %q", c.ext, got, c.want)
		}
		if got := rec.Header().Get("Cache-Control"); got != "public, max-age=86400" {
			t.Errorf("%s: Cache-Control %q", c.ext, got)
		}
	}
}

// logoRequest builds the multipart POST the screen sends. An empty filename
// means no file part at all, which is the ordinary save.
func logoRequest(t *testing.T, filename string, data []byte, fields ...string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for i := 0; i+1 < len(fields); i += 2 {
		if err := w.WriteField(fields[i], fields[i+1]); err != nil {
			t.Fatal(err)
		}
	}
	if filename != "" {
		part, err := w.CreateFormFile("logo", filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/marke", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}
