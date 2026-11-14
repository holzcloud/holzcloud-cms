package block

import (
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/value"
)

// A phone number is one of the two uses the Values screen names, and a block's
// button is where a tel: link stands (the Markdown sanitiser keeps none). The
// + of a global number has to reach the address as a +: in tel: it is syntax
// (RFC 3966), and %2B is another address (WR-02 of the second phase 33 review).
func TestAPhoneNumberValueReachesTheBlockLink(t *testing.T) {
	stored := Render([]Block{
		{Type: TypeCallout, Title: "Fragen?", LinkText: "Anrufen", LinkURL: "tel:[[wert:telefon]]"},
		{Type: TypeCallout, Title: "Schreiben", LinkText: "Mail", LinkURL: "mailto:[[wert:mail]]?subject=[[wert:betreff]]"},
	}, Builtin, images(nil), markdown)
	got := value.Resolve(stored, map[string]string{
		"telefon": "+41 44 123 45 67", "mail": "info+shop@x.ch", "betreff": "a&b",
	})
	for _, want := range []string{
		`href="tel:+41%2044%20123%2045%2067"`,
		`href="mailto:info+shop@x.ch?subject=a%26b"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("want %s in:\n%s\n(stored: %s)", want, got, stored)
		}
	}
}
