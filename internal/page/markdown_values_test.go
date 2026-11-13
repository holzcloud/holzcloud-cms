package page

import (
	"strings"
	"testing"
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
