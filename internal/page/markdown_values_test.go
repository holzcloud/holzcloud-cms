package page

import (
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/value"
)

// A value marker is stored literally wherever it stands, so that delivery has
// one spelling to look for (internal/value.Prefix).
func TestAValueMarkerInALinkIsStoredLiterally(t *testing.T) {
	const addr = "https://github.com/holzcloud/holzcloud-cms/releases/tag/v[[wert:version-holzcloud-cms]]"

	got, err := RenderMarkdown("[Release](" + addr + ")")
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	if !strings.Contains(got, `href="`+addr+`"`) {
		t.Errorf("Markdown link: the marker is not stored literally in the href:\n%s", got)
	}
	if strings.Contains(got, "%5B") {
		t.Errorf("Markdown link: the encoded spelling survived the save:\n%s", got)
	}

	auto, err := RenderMarkdown("Siehe " + addr + " dort.")
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	if !strings.Contains(auto, `href="`+addr+`"`) || !strings.Contains(auto, ">"+addr+"</a>") {
		t.Errorf("autolink: the marker is not stored literally in href and text:\n%s", auto)
	}
	if strings.Contains(auto, "%5B") {
		t.Errorf("autolink: the encoded spelling survived the save:\n%s", auto)
	}

	prose, err := RenderMarkdown("Alpha [[wert:v]]: neu")
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	if strings.TrimSpace(prose) != `<p>Alpha [[wert:v]]: neu</p>` {
		t.Errorf("prose with a marker changed: %q", prose)
	}
}

// A mail address is followed from the Markdown to the delivered address (WR-02
// of the second phase 33 review): a + in the address stays a +, and only the
// query encodes & = and +. (A tel: link is not Markdown's: the sanitiser keeps
// no tel: address, so a phone link is a block's, and internal/block follows it.)
func TestAMailAddressReachesItsLink(t *testing.T) {
	stored, err := RenderMarkdown("[schreiben](mailto:[[wert:mail]]?subject=[[wert:betreff]])")
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	got := value.Resolve(stored, map[string]string{"mail": "info+shop@x.ch", "betreff": "Frage & Antwort"})
	for _, want := range []string{
		`href="mailto:info+shop@x.ch?subject=Frage%20%26%20Antwort"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("want %s in:\n%s\n(stored: %s)", want, got, stored)
		}
	}
}

// The encoded spelling in prose is text the editor typed, not a marker: only
// a link address is restored (WR-02 of the phase 33 review).
func TestTheEncodedSpellingInProseStaysText(t *testing.T) {
	got, err := RenderMarkdown("Literal %5B%5Bwert:k%5D%5D in prose, `%5B%5Bwert:k%5D%5D` in code.")
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	if strings.Contains(got, "[[wert:") {
		t.Errorf("the encoded spelling in prose became a live marker:\n%s", got)
	}
}
